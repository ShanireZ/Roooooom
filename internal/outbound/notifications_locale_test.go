package outbound_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"roooooom.local/roooooom/internal/i18n"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
)

func setOwnerLanguage(t *testing.T, f *testkit.Fixture, language string) {
	t.Helper()
	if _, e := f.Core.Preferences(context.Background(), f.Owner(), model.Command{Kind: "owner.language", Payload: model.JSON(map[string]string{"language": language}), Meta: f.Meta(f.Owner())}); e != nil {
		t.Fatal(e)
	}
}

// riskNotification creates a high-risk task (which raises the system risk
// question) under a DingTalk binding with the given fields and returns the job.
func riskNotification(t *testing.T, f *testkit.Fixture, fields []string) outbound.Job {
	t.Helper()
	p, _ := f.Project()
	if e := privatefs.WriteAtomic(filepath.Dir(f.Store.Path), filepath.Join("credentials", "bot.json"), []byte(`{"token":"same-bot","secret":"SECsynthetic"}`)); e != nil {
		t.Fatal(e)
	}
	b := outbound.Binding{ProjectID: p, Kind: "dingtalk", CredentialRef: "bot", Enabled: true, Fields: fields, Events: []string{"decision-pending"}}
	if _, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())}); e != nil {
		t.Fatal(e)
	}
	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	return onlyJob(t, f)
}

func onlyJob(t *testing.T, f *testkit.Fixture) outbound.Job {
	t.Helper()
	var raw string
	if e := f.Store.QueryRow(context.Background(), "SELECT body FROM external_jobs WHERE kind='dingtalk.notify'").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var j outbound.Job
	if e := json.Unmarshal([]byte(raw), &j); e != nil {
		t.Fatal(e)
	}
	return j
}

func TestNotificationTextByOwnerLanguage(t *testing.T) {
	f := testkit.New(t)
	setOwnerLanguage(t, f, "ja")
	j := riskNotification(t, f, []string{"summary", "goal", "projectTitle"})
	for _, k := range []i18n.Key{i18n.NotifyDecision, i18n.NotifyReturn, i18n.LabelSummary, i18n.LabelTask, i18n.LabelProject} {
		if !strings.Contains(j.Text, i18n.T(i18n.JA, k)) {
			t.Errorf("job text lacks %s:\n%s", k, j.Text)
		}
	}
	if j.Notification == nil || j.Notification.Locale != "ja" || j.Notification.Summary == "" {
		t.Fatalf("notification locale/summary: %+v", j.Notification)
	}
}

func TestNotificationSummaryFieldToggle(t *testing.T) {
	for _, language := range []string{"en", "zh-TW"} {
		t.Run(language, func(t *testing.T) {
			f := testkit.New(t)
			setOwnerLanguage(t, f, language)
			j := riskNotification(t, f, []string{"goal"})
			if strings.Contains(j.Text, i18n.T(i18n.Locale(language), i18n.LabelSummary)) {
				t.Fatalf("summary included without the summary field:\n%s", j.Text)
			}
			if !strings.Contains(j.Text, i18n.T(i18n.Locale(language), i18n.LabelTask)) {
				t.Fatalf("goal label missing:\n%s", j.Text)
			}
		})
	}
}

func TestQueuedJobKeepsText(t *testing.T) {
	f := testkit.New(t)
	setOwnerLanguage(t, f, "ja")
	before := riskNotification(t, f, []string{"summary", "goal"})
	setOwnerLanguage(t, f, "en")
	if _, e := f.Store.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
		return model.Receipt{}, outbound.EnqueueNotifications(tx, f.Store.Identity().ServiceID, f.Store.Identity().Generation, before.TaskID)
	}); e != nil {
		t.Fatal(e)
	}
	if after := onlyJob(t, f); after.Text != before.Text || after.Notification.Locale != "ja" {
		t.Fatalf("queued job text changed after a language change:\n%s\n---\n%s", before.Text, after.Text)
	}
}

func deliveredContent(t *testing.T, locale string) string {
	t.Helper()
	var content string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v struct {
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		json.NewDecoder(r.Body).Decode(&v)
		content = v.Text.Content
		json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
	}))
	defer srv.Close()
	j := dingJob()
	j.Notification.Locale = locale
	if _, e := outbound.NewDingTalkForTest(srv.URL, srv.Client(), dingSecret, time.Now).Deliver(context.Background(), j); e != nil {
		t.Fatal(e)
	}
	return content
}

func TestLegacyJobFooter(t *testing.T) {
	if got := deliveredContent(t, ""); !strings.Contains(got, i18n.T(i18n.ZHTW, i18n.NotifyFooter)) || !strings.Contains(got, "NotificationId: notification-job") {
		t.Fatalf("legacy job footer:\n%s", got)
	}
	if got := deliveredContent(t, "ja"); !strings.Contains(got, i18n.T(i18n.JA, i18n.NotifyFooter)) {
		t.Fatalf("Japanese job footer:\n%s", got)
	}
}
