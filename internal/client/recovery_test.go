package client

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestExplicitRecoveryKeepsDraftsAndIdentity(t *testing.T) {
	f := testkit.New(t)
	project, _ := f.Project()
	a := f.Agent(project, "original")
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: server.Listener.Addr().String()})
	server.Start()
	defer server.Close()
	id := f.Store.Identity()
	root := filepath.Join(t.TempDir(), "profile")
	p := Profile{Root: root, Handle: "worker", ProjectID: project, ParticipantID: a.ParticipantID, SessionID: a.SessionID, ServiceURL: server.URL, ServiceID: id.ServiceID, Generation: id.Generation, Mode: "offline"}
	if e := SaveProfile(p); e != nil {
		t.Fatal(e)
	}
	c := New(p, nil)
	if _, e := c.Do(t.Context(), model.Command{Kind: "assignment.claim", TargetID: "offline-claim", Payload: model.JSON(struct{}{})}); e == nil {
		t.Fatal("draft silently committed")
	}
	c.Close()
	receipt, e := f.Identities.Control(t.Context(), f.Owner(), model.Command{Kind: "participant.recover", TargetID: a.ParticipantID, Meta: f.Meta(f.Owner()), Payload: model.JSON(struct{}{})})
	if e != nil {
		t.Fatal(e)
	}
	var grant struct{ CredentialPath string }
	json.Unmarshal(receipt.Result, &grant)
	got, e := AdoptRecovery(t.Context(), p, grant.CredentialPath, nil)
	if e != nil {
		t.Fatal(e)
	}
	if got.ParticipantID != p.ParticipantID || got.SessionID == p.SessionID || got.Mode != "reconciling" {
		t.Fatal(got)
	}
	c = New(got, nil)
	defer c.Close()
	items, e := c.Pending(t.Context())
	if e != nil || len(items) != 1 || items[0].Command.TargetID != "offline-claim" {
		t.Fatal("recovery lost draft", e)
	}
	wrong := p
	wrong.ServiceID = "another-service"
	if _, e = AdoptRecovery(t.Context(), wrong, grant.CredentialPath, nil); e == nil {
		t.Fatal("recovery accepted unrelated identity")
	}
}
