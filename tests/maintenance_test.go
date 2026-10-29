package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/cli"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"testing"
)

func TestOwnerMaintenanceAndRecovery(t *testing.T) {
	f, p, a, _ := recoveryFixture(t)
	handler := httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: "127.0.0.1:7337"})
	raw, e := os.ReadFile(filepath.Join(f.Identities.Root(), "credentials", "owner.json"))
	if e != nil {
		t.Fatal(e)
	}
	var owner struct{ Token, CSRF string }
	json.Unmarshal(raw, &owner)
	call := func(actor string, kind string, target model.ID, meta model.Mutation) *httptest.ResponseRecorder {
		body := model.JSON(struct {
			model.Mutation
			TargetID model.ID `json:"targetId"`
			Payload  any      `json:"payload"`
		}{meta, target, map[string]any{}})
		r := httptest.NewRequest("POST", "http://127.0.0.1:7337/api/v1/commands/"+kind, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://127.0.0.1:7337")
		if actor == "owner" {
			r.AddCookie(&http.Cookie{Name: "roooooom_owner", Value: owner.Token})
			r.Header.Set("X-CSRF-Token", owner.CSRF)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := call("none", "backup.create", "", f.Meta(f.Owner())); w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, kind := range []string{"backup.create", "backup.export"} {
		meta := f.Meta(f.Owner())
		w := call("owner", kind, "", meta)
		if w.Code != 200 {
			t.Fatal(kind, w.Code, w.Body.String())
		}
		w2 := call("owner", kind, "", meta)
		if w2.Code != 200 || w.Body.String() == "" {
			t.Fatal("backup retry failed", w2.Code)
		}
	}
	w := call("owner", "participant.recover", a.ParticipantID, f.Meta(f.Owner()))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response struct {
		Result struct{ CredentialPath string }
	}
	json.Unmarshal(w.Body.Bytes(), &response)
	credentials, e := os.ReadFile(response.Result.CredentialPath)
	if e != nil {
		t.Fatal(e)
	}
	var recovered struct {
		ParticipantID, SessionID model.ID
		Token                    string
	}
	json.Unmarshal(credentials, &recovered)
	actor, e := f.Identities.Authenticate(t.Context(), recovered.Token)
	if e != nil {
		t.Fatal(e)
	}
	if actor.ParticipantID != a.ParticipantID || actor.SessionID == a.SessionID || actor.ProjectIDs[0] != p {
		t.Fatal("recovery changed participant identity")
	}
	task := f.Task(f.Must(a, "task.create", "", testTaskPayload(p), f.Meta(a)).ResourceID)
	if task.IntegratorSessionID != a.SessionID {
		t.Fatal("recovery implicitly took over responsibilities")
	}
	var output bytes.Buffer
	if code := cli.Run(t.Context(), []string{"admin", "restore", "--data-dir", f.Identities.Root(), "--snapshot", "missing", "--confirm-service-id", "wrong"}, bytes.NewReader(nil), &output, &output); code == 0 {
		t.Fatal("restore ignored service confirmation")
	}
}
func testTaskPayload(p model.ID) map[string]any {
	return map[string]any{"projectId": p, "goal": "recovery", "scope": []string{"local"}, "nonScope": []string{"publish"}, "authorityRefs": []string{"synthetic"}, "risk": "normal", "acceptancePolicy": map[string]any{"mode": "integrator", "conditions": []string{"test"}}, "collaboratorLimit": 0}
}
