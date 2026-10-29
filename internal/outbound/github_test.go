package outbound_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"roooooom.local/roooooom/internal/outbound"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGitHubTimeoutAfterCreate(t *testing.T) {
	var creates atomic.Int32
	var visible atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r" {
			w.Write([]byte(`{"id":42,"full_name":"o/r","private":true}`))
			return
		}
		if r.Method == "POST" {
			creates.Add(1)
			h := w.(http.Hijacker)
			c, _, _ := h.Hijack()
			c.Close()
			return
		}
		if visible.Load() {
			w.Write([]byte(`[{"number":7,"html_url":"https://github.com/o/r/issues/7","body":"<!-- roooooom:synthetic -->"}]`))
		} else {
			w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()
	a := outbound.NewGitHubForTest(srv.URL, srv.Client(), func(string) ([]byte, error) { return []byte("synthetic-pat"), nil })
	j := outbound.Job{ID: "j", Kind: "github.issue", Marker: "<!-- roooooom:synthetic -->", Title: "local task", Text: "safe summary", Binding: outbound.Binding{Kind: "github", Repository: "o/r", RepositoryID: 42, Private: true, CredentialRef: "test"}}
	got, e := a.Deliver(context.Background(), j)
	if e != nil || got.State != "uncertain" || creates.Load() != 1 {
		t.Fatal(got, e, creates.Load())
	}
	got, e = a.Reconcile(context.Background(), j)
	if e != nil || got.State != "uncertain" || creates.Load() != 1 {
		t.Fatal("blind duplicate", got, e, creates.Load())
	}
	visible.Store(true)
	got, e = a.Reconcile(context.Background(), j)
	if e != nil || got.State != "delivered" || got.RemoteID != "7" || creates.Load() != 1 {
		t.Fatal(got, e, creates.Load())
	}
}
func TestGitHubBindingPinAndRateLimit(t *testing.T) {
	for _, code := range []int{201, 401, 403, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var writes atomic.Int32
			var wrong atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/repos/o/r" {
					id := 42
					if wrong.Load() {
						id = 43
					}
					json.NewEncoder(w).Encode(map[string]any{"id": id, "full_name": "o/r", "private": true})
					return
				}
				writes.Add(1)
				if code == 429 {
					w.Header().Set("Retry-After", "120")
				}
				w.WriteHeader(code)
				w.Write([]byte(`{"number":9,"html_url":"https://github.com/o/r/issues/9","message":"synthetic-pat"}`))
			}))
			defer srv.Close()
			a := outbound.NewGitHubForTest(srv.URL, srv.Client(), func(string) ([]byte, error) { return []byte("synthetic-pat"), nil })
			j := outbound.Job{Kind: "github.issue", Marker: "m", Title: "safe", Binding: outbound.Binding{Repository: "o/r", RepositoryID: 42, Private: true, CredentialRef: "test"}}
			before := time.Now()
			got, _ := a.Deliver(context.Background(), j)
			if code == 201 && got.State != "delivered" {
				t.Fatal(got)
			}
			if code == 429 && (got.State != "retry" || got.RetryAt.Before(before.Add(119*time.Second))) {
				t.Fatal(got)
			}
			if code == 500 && got.State != "uncertain" {
				t.Fatal(got)
			}
			if (code == 401 || code == 403) && got.State != "paused" {
				t.Fatal(got)
			}
			if strings.Contains(string(mustJSON(got)), "synthetic-pat") {
				t.Fatal("secret leaked")
			}
			wrong.Store(true)
			got, _ = a.Deliver(context.Background(), j)
			if got.State != "paused" || writes.Load() != 1 {
				t.Fatal("wrong repo written", got, writes.Load())
			}
		})
	}
}
func TestGitHubRedactionAndRedirect(t *testing.T) {
	for _, v := range []string{"Bearer abcdefghijklmnopqrstuvwxyzABCDEFGHIJK", "https://x.test/?access_token=secret秘密", "github_pat_abcdefghijklmnopqrstuvw", "safe\n秘密"} {
		clean := outbound.Redact(v)
		if strings.Contains(clean, "secret秘密") || strings.Contains(clean, "abcdefghijklmnopqrstuvw") {
			t.Fatal(clean)
		}
	}
	var calls atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer sink.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, sink.URL, 302) }))
	defer srv.Close()
	a := outbound.NewGitHubForTest(srv.URL, srv.Client(), func(string) ([]byte, error) { return []byte("synthetic"), nil })
	got, _ := a.Deliver(context.Background(), outbound.Job{Binding: outbound.Binding{Repository: "o/r", RepositoryID: 42, CredentialRef: "test"}})
	if got.State != "paused" || calls.Load() != 0 {
		t.Fatal("redirect followed", got, calls.Load())
	}
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func TestGitHubRemoteURLNeverExportsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`{"id":42,"full_name":"o/r","private":true}`))
			return
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"number":7,"html_url":"https://github.com/o/r/issues/7?access_token=do-not-export"}`))
	}))
	defer srv.Close()
	a := outbound.NewGitHubForTest(srv.URL, srv.Client(), func(string) ([]byte, error) { return []byte("synthetic"), nil })
	got, _ := a.Deliver(context.Background(), outbound.Job{Kind: "github.issue", Binding: outbound.Binding{Repository: "o/r", RepositoryID: 42, Private: true, CredentialRef: "test"}})
	if got.RemoteURL != "https://github.com/o/r/issues/7" {
		t.Fatal("unsafe remote URL", got.RemoteURL)
	}
}

func TestGitHubCredentialKindDoesNotCrossProviders(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write([]byte(`{"id":42,"full_name":"o/r","private":true}`))
	}))
	defer srv.Close()
	a := outbound.NewGitHubForTest(srv.URL, srv.Client(), func(string) ([]byte, error) {
		return []byte(`{"kind":"dingtalk","token":"do-not-send-to-github","secret":"SECsynthetic"}`), nil
	})
	a.Deliver(context.Background(), outbound.Job{Kind: "github.issue", Binding: outbound.Binding{Repository: "o/r", RepositoryID: 42, Private: true, CredentialRef: "test"}})
	if calls.Load() != 0 {
		t.Fatal("credential crossed integration kind", calls.Load())
	}
}
