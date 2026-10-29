package outbound_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
	"time"
)

func dingJob() outbound.Job {
	n, _ := outbound.BuildNotification("synthetic-request", 1, "decision-pending")
	return outbound.Job{ID: "notification-job", Kind: "dingtalk.notify", Notification: &n, Binding: outbound.Binding{Kind: "dingtalk", CredentialRef: "fixture", ChannelID: outbound.BotChannel("synthetic-token")}}
}
func dingSecret(string) ([]byte, error) {
	return []byte(`{"token":"synthetic-token","secret":"SECsynthetic秘密"}`), nil
}
func TestDingTalkSignatureAndBodyError(t *testing.T) {
	if got := outbound.Sign(1700000000123, "SECsynthetic秘密"); got != "xDDOJ%2FhU7E9X68eRAr8o2gR9p0XE42p1sjIBVzDKZkw%3D" {
		t.Fatal("fixed UTF-8 vector", got)
	}
	for _, code := range []int{0, 310000, 130101, -1, 40035} {
		t.Run(string(model.JSON(code)), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("access_token") != "synthetic-token" || r.URL.Query().Get("sign") == "" || r.URL.Query().Get("timestamp") == "" {
					t.Error("missing signed query")
				}
				var v map[string]any
				json.NewDecoder(r.Body).Decode(&v)
				if v["msgtype"] != "text" || v["at"].(map[string]any)["isAtAll"] != false {
					t.Error("payload", v)
				}
				json.NewEncoder(w).Encode(map[string]any{"errcode": code, "errmsg": "synthetic-token SECsynthetic秘密"})
			}))
			defer srv.Close()
			a := outbound.NewDingTalkForTest(srv.URL, srv.Client(), dingSecret, time.Now)
			got, e := a.Deliver(context.Background(), dingJob())
			if e != nil {
				t.Fatal(e)
			}
			if code == 0 && got.State != "accepted" {
				t.Fatal(got)
			}
			if code != 0 && got.State == "accepted" {
				t.Fatal("false acceptance", got)
			}
			if (code == 130101 || code == -1) && got.State != "retry" {
				t.Fatal(got)
			}
			if strings.Contains(string(mustJSON(got)), "synthetic-token") || strings.Contains(string(mustJSON(got)), "秘密") {
				t.Fatal("secret returned", got)
			}
		})
	}
	for _, skew := range []time.Duration{-2 * time.Hour, 2 * time.Hour} {
		t.Run(skew.String(), func(t *testing.T) {
			now := time.Now()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Date", now.Add(skew).UTC().Format(http.TimeFormat))
				w.Write([]byte(`{"errcode":0}`))
			}))
			defer srv.Close()
			a := outbound.NewDingTalkForTest(srv.URL, srv.Client(), dingSecret, func() time.Time { return now })
			got, _ := a.Deliver(context.Background(), dingJob())
			if got.State != "paused" || got.ErrorCode != "clock-skew" {
				t.Fatal(got)
			}
		})
	}
}
func TestNotificationTimeoutAndRedaction(t *testing.T) {
	clean := outbound.Redact("https://oapi.dingtalk.com/robot/send?access_token=synthetic&timestamp=123&sign=signature")
	if strings.Contains(clean, "access_token") || strings.Contains(clean, "timestamp") || strings.Contains(clean, "sign=") {
		t.Fatal("signed query was not fully redacted", clean)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { c, _, _ := w.(http.Hijacker).Hijack(); c.Close() }))
	defer srv.Close()
	a := outbound.NewDingTalkForTest(srv.URL, srv.Client(), dingSecret, time.Now)
	got, e := a.Deliver(context.Background(), dingJob())
	if e != nil || got.State != "uncertain" {
		t.Fatal(got, e)
	}
	if strings.Contains(string(mustJSON(got)), "access_token") || strings.Contains(string(mustJSON(got)), "synthetic-token") {
		t.Fatal("query leaked")
	}
	b := dingJob()
	b.Binding.ChannelID = outbound.BotChannel("different-bot")
	got, _ = a.Deliver(context.Background(), b)
	if got.State != "paused" {
		t.Fatal("rebound to another bot")
	}
	n, _ := outbound.BuildNotification("q", 2, "owner-blocker")
	if n.ID == "" || strings.Contains(n.Text, "http://127.0.0.1") {
		t.Fatal("mobile localhost promise", n)
	}
	if _, e = outbound.BuildNotification("q", 2, "ordinary-chat"); e == nil {
		t.Fatal("ordinary chat notification")
	}
}

type notificationAdapter struct {
	calls int
	fail  bool
}

func (a *notificationAdapter) Deliver(context.Context, outbound.Job) (outbound.Result, error) {
	a.calls++
	if a.fail {
		return outbound.Result{State: "uncertain", ErrorCode: "timeout"}, nil
	}
	return outbound.Result{State: "accepted"}, nil
}
func (a *notificationAdapter) Reconcile(ctx context.Context, j outbound.Job) (outbound.Result, error) {
	return a.Deliver(ctx, j)
}
func notifyBinding(t *testing.T, f *testkit.Fixture, p model.ID, ref string) {
	t.Helper()
	if e := privatefs.WriteAtomic(filepath.Dir(f.Store.Path), filepath.Join("credentials", ref+".json"), []byte(`{"token":"same-bot","secret":"SECsynthetic"}`)); e != nil {
		t.Fatal(e)
	}
	b := outbound.Binding{ProjectID: p, Kind: "dingtalk", CredentialRef: ref, Enabled: true, Fields: []string{"taskId", "goal"}, Events: []string{"decision-pending", "human-acceptance", "owner-blocker"}}
	if _, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())}); e != nil {
		t.Fatal(e)
	}
}
func TestResolvedNotificationSuppressed(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	notifyBinding(t, f, p, "bot")
	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	r := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	var id model.ID
	var version int64
	if e := f.Store.QueryRow(context.Background(), "SELECT id,version FROM decision_requests WHERE task_id=?", r.ResourceID).Scan(&id, &version); e != nil {
		t.Fatal(e)
	}
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = version
	f.Must(f.Owner(), "decision.withdraw", id, map[string]any{"reason": "no longer needed"}, meta)
	a := &notificationAdapter{}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"dingtalk": a})
	if e := q.RunOnce(context.Background(), time.Now()); e != nil {
		t.Fatal(e)
	}
	if a.calls != 0 {
		t.Fatal("sent after resolution", a.calls)
	}
	var count int
	f.Store.QueryRow(context.Background(), "SELECT count(*) FROM external_jobs WHERE status='canceled'").Scan(&count)
	if count != 1 {
		t.Fatal("stale notification not durably suppressed", count)
	}
}

func TestResolvedNotificationPausedBacklogSuppressed(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	notifyBinding(t, f, p, "bot")
	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	r := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	var id model.ID
	var version int64
	if e := f.Store.QueryRow(context.Background(), "SELECT id,version FROM decision_requests WHERE task_id=?", r.ResourceID).Scan(&id, &version); e != nil {
		t.Fatal(e)
	}
	_, e := f.Store.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("UPDATE external_jobs SET status='paused'")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = version
	f.Must(f.Owner(), "decision.withdraw", id, map[string]any{"reason": "already handled"}, meta)
	a := &notificationAdapter{}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"dingtalk": a})
	if e = q.RunOnce(context.Background(), time.Now()); e != nil {
		t.Fatal(e)
	}
	var status string
	f.Store.QueryRow(context.Background(), "SELECT status FROM external_jobs LIMIT 1").Scan(&status)
	if status != "canceled" || a.calls != 0 {
		t.Fatal("resolved paused backlog not suppressed", status, a.calls)
	}
}
func TestBotGlobalBudget(t *testing.T) {
	f := testkit.New(t)
	for i := 0; i < 2; i++ {
		p, _ := f.Project()
		notifyBinding(t, f, p, "bot-"+string(rune('a'+i)))
		for j := 0; j < 8; j++ {
			payload := testkit.TaskPayload(p, 0)
			payload.Risk = "high"
			f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
		}
	}
	a := &notificationAdapter{}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"dingtalk": a})
	start := time.Now()
	for i := 0; i < 60; i++ {
		if e := q.RunOnce(context.Background(), start.Add(time.Duration(i)*time.Second)); e != nil {
			t.Fatal(e)
		}
	}
	if a.calls != 10 {
		t.Fatal("shared bot budget", a.calls)
	}
}
func TestNotificationRetryBudgetAndDedupe(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	notifyBinding(t, f, p, "bot")
	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	r := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	_, e := f.Store.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
		return model.Receipt{}, outbound.EnqueueNotifications(tx, f.Store.Identity().ServiceID, f.Store.Identity().Generation, r.ResourceID)
	})
	if e != nil {
		t.Fatal(e)
	}
	var count int
	f.Store.QueryRow(context.Background(), "SELECT count(*) FROM external_jobs").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate jobs", count)
	}
	a := &notificationAdapter{fail: true}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"dingtalk": a})
	start := time.Now()
	for i := 0; i < 400; i++ {
		if e := q.RunOnce(context.Background(), start.Add(time.Duration(i)*time.Second)); e != nil {
			t.Fatal(e)
		}
	}
	if a.calls != 3 {
		t.Fatal("unbounded notification retry", a.calls)
	}
	var status string
	f.Store.QueryRow(context.Background(), "SELECT status FROM external_jobs").Scan(&status)
	if status != "paused" {
		t.Fatal(status)
	}
}
