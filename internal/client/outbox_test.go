package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func profile(t *testing.T) Profile {
	t.Helper()
	p := Profile{Root: filepath.Join(t.TempDir(), "client"), Handle: "synthetic", ServiceURL: "http://127.0.0.1:7337", ServiceID: "service", Generation: "generation", ProjectID: "project", ParticipantID: "participant", SessionID: "session", CredentialRef: "credentials/session.json", Mode: "online"}
	if e := privatefs.WriteAtomic(p.Root, p.CredentialRef, model.JSON(map[string]string{"token": "FAKE_SECRET_SENTINEL"})); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestOfflineFirstUseAndDurableDraft(t *testing.T) {
	p := profile(t)
	p.Mode = "offline"
	p.Provisional = true
	p.ServiceID = ""
	p.Generation = ""
	c := New(p, roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, errors.New("synthetic outage") }))
	_, e := c.Do(context.Background(), model.Command{Kind: "assignment.claim", TargetID: "assignment", Meta: model.Mutation{ExpectedVersion: 1}, Payload: model.JSON(struct{}{})})
	if e == nil {
		t.Fatal("offline claim falsely succeeded")
	}
	c.Close()
	next := New(p, nil)
	defer next.Close()
	drafts, e := next.Pending(context.Background())
	if e != nil || len(drafts) != 1 {
		t.Fatal("lost durable draft", drafts, e)
	}
	if drafts[0].Authoritative || drafts[0].RequestID == "" || drafts[0].Command.Meta.ExpectedVersion != 1 {
		t.Fatal(drafts)
	}
	raw := model.JSON(drafts)
	if strings.Contains(string(raw), "FAKE_SECRET_SENTINEL") {
		t.Fatal("credential in outbox")
	}
}
func TestReconcileDisconnectAndChangedEpoch(t *testing.T) {
	p := profile(t)
	attempts := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/identity" {
			return response(200, model.JSON(map[string]any{"status": "ok", "serviceId": p.ServiceID, "datasetGeneration": p.Generation, "protocolVersion": 1})), nil
		}
		attempts++
		return nil, io.ErrUnexpectedEOF
	})
	c := New(p, transport)
	defer c.Close()
	cmd := model.Command{Kind: "task.complete", TargetID: "task", Meta: model.Mutation{ExpectedVersion: 7, IntegratorEpoch: 4}, Payload: model.JSON(map[string]string{"candidateId": "candidate"})}
	if _, e := c.Do(context.Background(), cmd); e == nil {
		t.Fatal("response loss falsely acknowledged")
	}
	drafts, e := c.Pending(context.Background())
	if e != nil || len(drafts) != 1 {
		t.Fatal(e)
	}
	first := drafts[0]
	if _, e = c.Reconcile(context.Background()); e == nil {
		t.Fatal("reconcile outage ignored")
	}
	drafts, e = c.Pending(context.Background())
	if e != nil || len(drafts) != 1 || drafts[0].RequestID != first.RequestID || drafts[0].Command.Meta.IntegratorEpoch != 4 {
		t.Fatal("outbox changed or lost", e)
	}
	if attempts < 1 {
		t.Fatal("HTTP entry not used")
	}
}

type stepClock struct{ now time.Time }

func (s *stepClock) Now() time.Time { return s.now }
func response(status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}
}
func TestWaitBudgetAndNoSecretOutput(t *testing.T) {
	p := profile(t)
	clock := &stepClock{now: time.Now().UTC()}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/identity" {
			return response(200, model.JSON(map[string]any{"status": "ok", "serviceId": p.ServiceID, "datasetGeneration": p.Generation, "protocolVersion": 1})), nil
		}
		clock.now = clock.now.Add(time.Minute)
		return response(200, model.JSON(map[string]any{"result": map[string]any{"events": []any{}, "cursor": "", "hasMore": false, "serverTime": clock.now, "connection": "online"}})), nil
	})
	c := New(p, transport)
	c.clock = clock
	defer c.Close()
	exhausted := false
	for range 11 {
		_, e := c.Wait(context.Background(), time.Minute)
		if e != nil {
			var d *model.DomainError
			if errors.As(e, &d) && d.Code == "wait-budget-exhausted" {
				exhausted = true
				break
			}
			t.Fatal(e)
		}
	}
	if !exhausted || clock.now.Sub(time.Now()) > 11*time.Minute {
		t.Fatal("unbounded wait")
	}
	raw, _ := json.Marshal(c.Profile)
	if strings.Contains(string(raw), "FAKE_SECRET_SENTINEL") {
		t.Fatal("secret output")
	}
}
