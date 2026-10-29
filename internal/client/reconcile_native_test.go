package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestRecoveredReceiptDoesNotReapplyChangedClaim(t *testing.T) {
	f := testkit.New(t)
	project, repo := f.Project()
	integrator := f.Agent(project, "integrator")
	task := f.Must(integrator, "task.create", "", testkit.TaskPayload(project, 1), f.Meta(integrator))
	meta := f.Meta(integrator)
	meta.IntegratorEpoch = 1
	assignment := f.Must(integrator, "assignment.create", "", testkit.AssignmentPayload(task.ResourceID, repo), meta)
	cap, e := f.Identities.EnableEnrollment(t.Context(), f.Owner(), project)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.Identities.Enroll(t.Context(), cap, "worker", "worker")
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: server.Listener.Addr().String()})
	server.Start()
	defer server.Close()
	root := filepath.Join(t.TempDir(), "client")
	if e = privatefs.WriteAtomic(root, "credentials/session.json", model.JSON(credentials)); e != nil {
		t.Fatal(e)
	}
	identity := f.Store.Identity()
	p := Profile{Root: root, Handle: "worker", ServiceURL: server.URL, ServiceID: identity.ServiceID, Generation: identity.Generation, ProjectID: project, ParticipantID: credentials.ParticipantID, SessionID: credentials.SessionID, CredentialRef: "credentials/session.json", Mode: "online"}
	dropped := false
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, e := http.DefaultTransport.RoundTrip(r)
		if e == nil && !dropped && r.URL.Path == "/api/v1/commands/assignment.claim" && response.StatusCode == 200 {
			dropped = true
			response.Body.Close()
			return nil, io.ErrUnexpectedEOF
		}
		return response, e
	})
	c := New(p, transport)
	defer c.Close()
	if _, e = c.Do(t.Context(), model.Command{Kind: "assignment.claim", TargetID: assignment.ResourceID, Meta: model.Mutation{ExpectedVersion: 1}, Payload: model.JSON(struct{}{})}); e == nil {
		t.Fatal("synthetic response loss not observed")
	}
	meta = f.Meta(f.Owner())
	meta.ExpectedVersion = 2
	meta.ClaimEpoch = 1
	f.Must(f.Owner(), "assignment.release", assignment.ResourceID, map[string]any{"reason": "explicit reassignment", "exitTask": true}, meta)
	meta = f.Meta(integrator)
	meta.ExpectedVersion = 3
	f.Must(integrator, "assignment.claim", assignment.ResourceID, struct{}{}, meta)
	before := f.Assignment(assignment.ResourceID)
	results, e := c.Reconcile(t.Context())
	if e != nil || len(results) != 1 || results[0].State != "committed" || results[0].Receipt == nil {
		t.Fatal(results, e)
	}
	after := f.Assignment(assignment.ResourceID)
	if before.Version != after.Version || after.ClaimEpoch != 3 || after.ClaimParticipantID != integrator.ParticipantID {
		t.Fatal("historical receipt reapplied a claim")
	}
	drafts, e := c.Pending(t.Context())
	if e != nil || len(drafts) != 0 {
		t.Fatal("confirmed draft not removed", e)
	}
}
func TestServiceIdentityMismatchNeverSendsCredentials(t *testing.T) {
	p := profile(t)
	credentialSent := false
	c := New(p, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" {
			credentialSent = true
		}
		return response(200, model.JSON(map[string]any{"status": "ok", "serviceId": "different", "datasetGeneration": p.Generation, "protocolVersion": 1})), nil
	}))
	defer c.Close()
	_, e := c.Do(context.Background(), model.Command{Kind: "assignment.claim", TargetID: "assignment", Meta: model.Mutation{ExpectedVersion: 1}, Payload: model.JSON(struct{}{})})
	if model.Status(e) != 409 || credentialSent {
		t.Fatal("credential sent to a different service", e)
	}
}
func TestOutboxRetryKeepsOriginalRequestAndRemovesConfirmedItem(t *testing.T) {
	p := profile(t)
	c := New(p, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/identity" {
			return response(200, model.JSON(map[string]any{"status": "ok", "serviceId": p.ServiceID, "datasetGeneration": p.Generation, "protocolVersion": 1})), nil
		}
		var body struct {
			ClientRequestID string `json:"clientRequestId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		return response(200, model.JSON(map[string]any{"requestId": body.ClientRequestID, "result": map[string]any{"id": "task"}, "objectVersions": map[string]int64{"task": 1}, "committedEventId": 1})), nil
	}))
	defer c.Close()
	cmd := model.Command{Kind: "task.create", Meta: model.Mutation{ClientRequestID: "same"}, Payload: model.JSON(testkit.TaskPayload(p.ProjectID, 0))}
	c.Profile.Mode = "offline"
	if _, e := c.Do(t.Context(), cmd); e == nil {
		t.Fatal("draft not staged")
	}
	c.Profile.Mode = "online"
	if _, e := c.Do(t.Context(), cmd); e != nil {
		t.Fatal(e)
	}
	pending, e := c.Pending(t.Context())
	if e != nil || len(pending) != 0 {
		t.Fatal("retry removed the wrong local UUID", e)
	}
}
