package outbound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuidedGitHubInspection(t *testing.T) {
	calls := 0
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/repos/Owner/Repo" || r.Header.Get("Authorization") != "Bearer synthetic-pat" {
			t.Error("unexpected metadata request")
		}
		if calls == 3 {
			w.Header().Set("Location", "/leaked")
			w.WriteHeader(301)
			return
		}
		if calls == 4 {
			w.WriteHeader(403)
			_, _ = w.Write([]byte("synthetic-pat secret remote error"))
			return
		}
		_, _ = w.Write([]byte(`{"id":42,"full_name":"Owner/Repo","private":true,"has_issues":true}`))
	}))
	defer remote.Close()
	g := NewGitHubForTest(remote.URL, remote.Client(), func(string) ([]byte, error) { return []byte("synthetic-pat"), nil })
	for _, reference := range []string{"Owner/Repo", "https://github.com/Owner/Repo.git"} {
		v, e := g.Inspect(context.Background(), reference, "private-ref")
		if e != nil || v.RepositoryID != 42 || !v.Private || !v.HasIssues || v.IssuesWriteVerified {
			t.Fatalf("%+v %v", v, e)
		}
	}
	for _, reference := range []string{"https://user@github.com/Owner/Repo", "https://github.com:443/Owner/Repo", "https://evil.test/Owner/Repo", "https://github.com/Owner/Repo#x", "https://github.com/Owner/../Repo", "Owner/.."} {
		if _, e := g.Inspect(context.Background(), reference, "private-ref"); e == nil {
			t.Fatal(reference)
		}
	}
	if calls != 2 {
		t.Fatal("invalid URL reached network")
	}
	if _, e := g.Inspect(context.Background(), "Owner/Repo", "private-ref"); e == nil {
		t.Fatal("followed redirect")
	}
	if calls != 3 {
		t.Fatal(calls)
	}
	if _, e := g.Inspect(context.Background(), "Owner/Repo", "private-ref"); e == nil || strings.Contains(e.Error(), "synthetic-pat") {
		t.Fatal("remote response leaked", e)
	}
}

func TestGuidedWebhookCredential(t *testing.T) {
	token, e := ParseWebhook("https://oapi.dingtalk.com/robot/send?access_token=synthetic-token")
	if e != nil || token != "synthetic-token" {
		t.Fatal(token, e)
	}
	for _, url := range []string{"http://oapi.dingtalk.com/robot/send?access_token=a", "https://u@oapi.dingtalk.com/robot/send?access_token=a", "https://oapi.dingtalk.com:443/robot/send?access_token=a", "https://evil.test/robot/send?access_token=a", "https://oapi.dingtalk.com/robot/send?access_token=a#x", "https://oapi.dingtalk.com/robot/send?access_token=a&x=b", "https://oapi.dingtalk.com/robot/send?access_token=a&access_token=b", "https://oapi.dingtalk.com/robot/../send?access_token=a", "https://oapi.dingtalk.com/robot/send?access_token=a%20b"} {
		if _, e := ParseWebhook(url); e == nil {
			t.Fatal(url)
		}
	}
}
