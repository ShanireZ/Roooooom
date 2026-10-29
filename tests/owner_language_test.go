package tests

import (
	"context"
	"encoding/json"
	"testing"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

func (f *authFixture) me(t *testing.T, token string) map[string]any {
	t.Helper()
	w := f.request("GET", "/api/v1/me", "", token, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var envelope struct {
		Result map[string]any `json:"result"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	return envelope.Result
}

func TestMeReportsOwnerLanguageAndBuild(t *testing.T) {
	f := newAuthFixture(t)
	owner := f.me(t, "owner")
	if language, ok := owner["ownerLanguage"]; !ok || language != "" {
		t.Fatalf("initial ownerLanguage %v, %v", language, ok)
	}
	if build, ok := owner["build"].(map[string]any); !ok || build["goVersion"] == nil {
		t.Fatalf("build info missing: %v", owner["build"])
	}
	id := f.store.Identity()
	raw, _ := json.Marshal(map[string]any{"serviceId": id.ServiceID, "datasetGeneration": id.Generation, "clientRequestId": "language", "payload": map[string]any{"language": "zh-TW"}})
	if w := f.request("POST", "/api/v1/commands/owner.language", string(raw), "owner", "http://127.0.0.1:7337"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if language := f.me(t, "owner")["ownerLanguage"]; language != "zh-TW" {
		t.Fatalf("ownerLanguage after set %v", language)
	}
}

func TestAgentCannotReadOrWriteOwnerLanguage(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	if _, e := f.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	}); e != nil {
		t.Fatal(e)
	}
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	capability, e := f.ids.EnableEnrollment(ctx, owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := f.ids.Enroll(ctx, capability, "agent", "worker")
	if e != nil {
		t.Fatal(e)
	}
	id := f.store.Identity()
	raw, _ := json.Marshal(map[string]any{"serviceId": id.ServiceID, "datasetGeneration": id.Generation, "clientRequestId": "language", "payload": map[string]any{"language": "ja"}})
	if w := f.request("POST", "/api/v1/commands/owner.language", string(raw), agent.Token, ""); w.Code != 403 {
		t.Fatal("agent wrote owner language", w.Code, w.Body.String())
	}
	result := f.me(t, agent.Token)
	if _, ok := result["ownerLanguage"]; ok {
		t.Fatal("agent read owner language")
	}
	if _, ok := result["build"]; ok {
		t.Fatal("agent read owner build info")
	}
}
