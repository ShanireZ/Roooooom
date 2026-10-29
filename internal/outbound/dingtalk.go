package outbound

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DingTalk struct {
	base       string
	client     *http.Client
	resolve    func(string) ([]byte, error)
	now        func() time.Time
	mu         sync.Mutex
	lastMillis int64
}
type BotCredential struct {
	Kind   string `json:"kind,omitempty"`
	Token  string `json:"token"`
	Secret string `json:"secret"`
}

func BotChannel(token string) string { return "dingtalk:" + digest(token) }
func Sign(timestampMillis int64, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(strconv.FormatInt(timestampMillis, 10) + "\n" + secret))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}
func NewDingTalk(resolve func(string) ([]byte, error)) *DingTalk {
	return NewDingTalkForTest("https://oapi.dingtalk.com/robot/send", &http.Client{Timeout: 15 * time.Second}, resolve, time.Now)
}
func NewDingTalkForTest(base string, c *http.Client, resolve func(string) ([]byte, error), now func() time.Time) *DingTalk {
	copy := *c
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &DingTalk{base: base, client: &copy, resolve: resolve, now: now}
}
func ReadBot(resolve func(string) ([]byte, error), ref string) (BotCredential, error) {
	var c BotCredential
	b, e := resolve(ref)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil || c.Kind != "" && c.Kind != "dingtalk" || c.Token == "" || c.Secret == "" || len(c.Token) > 512 || len(c.Secret) > 512 || strings.ContainsAny(c.Token, "\r\n ") {
		return BotCredential{}, io.ErrUnexpectedEOF
	}
	return c, nil
}
func (d *DingTalk) Deliver(ctx context.Context, j Job) (Result, error) {
	credential, e := ReadBot(d.resolve, j.Binding.CredentialRef)
	if e != nil {
		return Result{State: "paused", ErrorCode: "private-bot-credential-unavailable"}, nil
	}
	if j.Binding.ChannelID != BotChannel(credential.Token) {
		return Result{State: "paused", ErrorCode: "bot-binding-mismatch"}, nil
	}
	if j.Notification == nil {
		return Result{State: "paused", ErrorCode: "missing-notification"}, nil
	}
	now := d.now()
	millis := now.UnixMilli()
	d.mu.Lock()
	if d.lastMillis-millis > 5*60*1000 {
		d.mu.Unlock()
		return Result{State: "paused", ErrorCode: "clock-skew"}, nil
	}
	if millis <= d.lastMillis {
		millis = d.lastMillis + 1
	}
	d.lastMillis = millis
	d.mu.Unlock()
	target := d.base + "?access_token=" + url.QueryEscape(credential.Token) + "&timestamp=" + strconv.FormatInt(millis, 10) + "&sign=" + Sign(millis, credential.Secret)
	// Text-only notification is the default fallback. No URLs, markdown links,
	// mentions, inbound callback, or claims of mobile access to localhost.
	text := j.Notification.Text + "\nNotificationId: " + string(j.ID) + "\n可能因網路結果未知而重複；請以本機決策頁目前狀態為準。"
	text = strings.ReplaceAll(strings.ReplaceAll(text, credential.Token, "[redacted]"), credential.Secret, "[redacted]")
	payload := map[string]any{"msgtype": "text", "text": map[string]string{"content": text}, "at": map[string]any{"isAtAll": false, "atUserIds": []string{}, "atMobiles": []string{}}}
	req, e := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(mustBytes(payload)))
	if e != nil {
		return Result{State: "paused", ErrorCode: "invalid-notification"}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	r, e := d.client.Do(req)
	if e != nil {
		return Result{State: "uncertain", ErrorCode: "notification-outcome-unknown"}, nil
	}
	defer r.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(r.Body, 65537))
	if e != nil || len(raw) > 65536 {
		return Result{State: "uncertain", ErrorCode: "notification-response-unavailable"}, nil
	}
	if remote, e := http.ParseTime(r.Header.Get("Date")); e == nil && (remote.Sub(now) > time.Hour || now.Sub(remote) > time.Hour) {
		return Result{State: "paused", ErrorCode: "clock-skew"}, nil
	}
	if r.StatusCode == 429 {
		rate, _ := rateResult(r)
		return rate, nil
	}
	if r.StatusCode >= 500 {
		return Result{State: "retry", ErrorCode: "notification-platform-unavailable"}, nil
	}
	if r.StatusCode != 200 {
		return Result{State: "paused", ErrorCode: "notification-http-rejected"}, nil
	}
	var response struct {
		Code *int `json:"errcode"`
	}
	if json.Unmarshal(raw, &response) != nil || response.Code == nil {
		return Result{State: "uncertain", ErrorCode: "invalid-notification-response"}, nil
	}
	switch *response.Code {
	case 0:
		return Result{State: "accepted"}, nil
	case 130101:
		return Result{State: "retry", ErrorCode: "notification-platform-rate-limit", RetryAt: now.Add(time.Minute)}, nil
	case -1:
		return Result{State: "retry", ErrorCode: "notification-platform-unavailable"}, nil
	case 310000:
		return Result{State: "paused", ErrorCode: "clock-signature-or-security-policy"}, nil
	default:
		return Result{State: "paused", ErrorCode: "notification-platform-rejected"}, nil
	}
}

// The webhook has no read-back reconciliation API. Bounded retries keep the
// same NotificationId and may duplicate an alert, never a formal decision.
func (d *DingTalk) Reconcile(ctx context.Context, j Job) (Result, error) { return d.Deliver(ctx, j) }
