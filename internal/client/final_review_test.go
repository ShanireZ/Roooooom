package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
)

func finalReviewClient(t *testing.T, f *testkit.Fixture, p model.ID) *Client {
	t.Helper()
	cap, e := f.Identities.EnableEnrollment(t.Context(), f.Owner(), p)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.Identities.Enroll(t.Context(), cap, "review-profile", "review-profile")
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: server.Listener.Addr().String()})
	server.Start()
	t.Cleanup(server.Close)
	root := filepath.Join(t.TempDir(), "client")
	if e = privatefs.WriteAtomic(root, "credentials/session.json", model.JSON(credentials)); e != nil {
		t.Fatal(e)
	}
	identity := f.Store.Identity()
	c := New(Profile{Root: root, Handle: "review", ServiceURL: server.URL, ServiceID: identity.ServiceID, Generation: identity.Generation, ProjectID: p, ParticipantID: credentials.ParticipantID, SessionID: credentials.SessionID, CredentialRef: "credentials/session.json", Mode: "online"}, nil)
	t.Cleanup(func() { c.Close() })
	return c
}
func TestFinalInvalidDraftRejectedBeforePersistence(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	c := finalReviewClient(t, f, p)
	_, e := c.Do(t.Context(), model.Command{Kind: "task.create", Payload: model.JSON(map[string]any{"unknown": true})})
	if model.Status(e) != 400 {
		t.Fatal("invalid command not rejected", e)
	}
	pending, e := c.Pending(t.Context())
	if e != nil || len(pending) != 0 {
		t.Fatal("known syntax error retained as pending operation", e)
	}
}
func TestFinalLegacyInvalidDraftDoesNotBlockReconcile(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	c := finalReviewClient(t, f, p)
	bad := model.Command{Kind: "task.create", Meta: c.meta(model.Mutation{}), Payload: model.JSON(map[string]any{"unknown": true})}
	item := OutboxItem{LocalID: "legacy-invalid", RequestID: bad.Meta.ClientRequestID, ProjectID: p, Command: bad, LocalOccurredAt: c.clock.Now()}
	if _, e := c.put(t.Context(), item); e != nil {
		t.Fatal(e)
	}
	c.Profile.Mode = "offline"
	_, e := c.Do(t.Context(), model.Command{Kind: "task.create", Payload: model.JSON(testkit.TaskPayload(p, 0))})
	if e == nil {
		t.Fatal("offline draft not staged")
	}
	profile := c.Profile
	profile.Mode = "online"
	c.Close()
	c = New(profile, nil)
	defer c.Close()
	results, e := c.Reconcile(t.Context())
	if e != nil || len(results) != 2 || results[0].State != "rejected" || results[1].State != "candidate" {
		t.Fatal("invalid draft blocked subsequent valid draft after restart", results, e)
	}
	pending, e := c.Pending(t.Context())
	if e != nil || len(pending) != 1 || pending[0].LocalID != "legacy-invalid" || c.Profile.Mode != "reconciling" {
		t.Fatal("invalid original bytes lost or falsely completed", pending, e)
	}
	if _, e = c.Reconcile(t.Context()); e != nil {
		t.Fatal("quarantined draft poisoned next reconciliation", e)
	}
}
func TestFinalLargeValidRecordsPageAndReconcile(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "creator")
	c := finalReviewClient(t, f, p)
	for range 3 {
		payload := testkit.TaskPayload(p, 0)
		payload.Scope = make([]string, 200)
		for i := range payload.Scope {
			payload.Scope[i] = strings.Repeat("s", 4096)
		}
		if len(model.JSON(payload)) >= 1<<20 {
			t.Fatal("fixture request exceeds real input bound")
		}
		f.Must(a, "task.create", "", payload, f.Meta(a))
	}
	cursor := ""
	seen := map[model.ID]bool{}
	for page := 0; ; page++ {
		if page > 10 {
			t.Fatal("cursor failed to advance")
		}
		v, e := c.Query(t.Context(), model.Query{Kind: "tasks", ProjectID: p, Limit: 100, Cursor: cursor})
		if e != nil {
			t.Fatal("healthy service unreadable", e)
		}
		var tasks []model.Task
		if json.Unmarshal(v.Data, &tasks) != nil {
			t.Fatal("bad task page")
		}
		if page == 0 && !v.HasMore {
			t.Fatal("large page lacks byte budget")
		}
		for _, task := range tasks {
			if seen[task.ID] {
				t.Fatal("duplicate cursor record")
			}
			seen[task.ID] = true
		}
		if !v.HasMore {
			break
		}
		if cursor == v.Cursor || len(tasks) == 0 {
			t.Fatal("non-progressing byte page")
		}
		cursor = v.Cursor
	}
	if len(seen) != 3 {
		t.Fatal("byte paging lost valid records")
	}
	if _, e := c.Reconcile(t.Context()); e != nil {
		t.Fatal("snapshot blocks reconcile", e)
	}
}
func TestFinalOversizedSingleRecordIsExplicit(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "creator")
	c := finalReviewClient(t, f, p)
	payload := testkit.TaskPayload(p, 0)
	payload.Scope = make([]string, 200)
	for i := range payload.Scope {
		payload.Scope[i] = strings.Repeat("s", 4096)
	}
	task := f.Must(a, "task.create", "", payload, f.Meta(a))
	policy := payload.AcceptancePolicy
	policy.Conditions = make([]string, 200)
	for i := range policy.Conditions {
		policy.Conditions[i] = fmt.Sprint(i) + strings.Repeat("c", 4090)
	}
	m := f.Meta(f.Owner())
	m.ExpectedVersion = 1
	f.Must(f.Owner(), "acceptance.policy", task.ResourceID, map[string]any{"policy": policy, "reason": "individually valid large policy"}, m)
	_, e := c.Query(t.Context(), model.Query{Kind: "task", TargetID: task.ResourceID})
	var de *model.DomainError
	if !errors.As(e, &de) || de.Status != 413 || de.Code != "response-too-large" {
		t.Fatal("oversized response misreported as service offline", e)
	}
}
