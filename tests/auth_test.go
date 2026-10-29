package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
)

func TestHTTPTaskClaimAndRead(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	_, e := f.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(ctx, owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := f.ids.Enroll(ctx, cap, "worker", "worker")
	if e != nil {
		t.Fatal(e)
	}
	id := f.store.Identity()
	body := map[string]any{"serviceId": id.ServiceID, "datasetGeneration": id.Generation, "clientRequestId": "task-create", "sessionId": agent.SessionID, "payload": testkit.TaskPayload("p", 0)}
	b, _ := json.Marshal(body)
	w := f.request("POST", "/api/v1/commands/task.create", string(b), agent.Token, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var receipt struct{ Result model.Task }
	if e = json.Unmarshal(w.Body.Bytes(), &receipt); e != nil {
		t.Fatal(e)
	}
	w = f.request("GET", "/api/v1/queries/task?targetId="+string(receipt.Result.ID), "", agent.Token, "")
	if w.Code != 200 {
		t.Fatal("query", w.Code, w.Body.String())
	}
	var view struct{ Result model.Task }
	if e = json.Unmarshal(w.Body.Bytes(), &view); e != nil {
		t.Fatal(e)
	}
	if !view.Result.CanStart || view.Result.CollaboratorLimit != 0 || view.Result.IntegratorID != agent.ParticipantID {
		t.Fatal(view)
	}
	for _, kind := range []string{"task.create", "integrator.claim", "assignment.create", "assignment.claim", "assignment.release", "assignment.update", "collaborator.limit", "task.reopen", "task.cancel"} {
		if w := f.request("POST", "/api/v1/commands/"+kind, "{}", "", ""); w.Code != 401 {
			t.Fatal(kind, "unauthenticated", w.Code)
		}
		if w := f.request("POST", "/api/v1/commands/"+kind, strings.Repeat("x", 2048), "owner", "http://127.0.0.1:7337"); w.Code != 413 {
			t.Fatal(kind, "body budget", w.Code)
		}
	}
}
func TestHTTPHandoffRejectsOldEpoch(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	_, e := f.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(ctx, owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	a, e := f.ids.Enroll(ctx, cap, "a", "integrator")
	if e != nil {
		t.Fatal(e)
	}
	b, e := f.ids.Enroll(ctx, cap, "b", "next")
	if e != nil {
		t.Fatal(e)
	}
	serviceIdentity := f.store.Identity()
	send := func(actor identity.Credentials, kind string, target model.ID, payload any, version, epoch int64) map[string]any {
		t.Helper()
		raw := model.JSON(map[string]any{"serviceId": serviceIdentity.ServiceID, "datasetGeneration": serviceIdentity.Generation, "clientRequestId": string((model.RandomIDs{}).NewID()), "sessionId": actor.SessionID, "targetId": target, "expectedVersion": version, "integratorEpoch": epoch, "payload": payload})
		w := f.request("POST", "/api/v1/commands/"+kind, string(raw), actor.Token, "")
		if w.Code != 200 {
			t.Fatal(kind, w.Code, w.Body.String())
		}
		var response struct{ Result map[string]any }
		if e = json.Unmarshal(w.Body.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		return response.Result
	}
	task := send(a, "task.create", "", testkit.TaskPayload("p", 0), 0, 0)
	taskID := model.ID(task["id"].(string))
	payload := map[string]any{"fromSessionId": a.SessionID, "toSessionId": b.SessionID, "reason": "explicit transfer", "dispositions": []any{}}
	offer := send(a, "integrator.handoff", taskID, payload, 1, 1)
	payload["handoffId"] = offer["handoffId"]
	payload["acceptedHandoffVersion"] = offer["version"]
	send(b, "integrator.handoff", taskID, payload, 1, 1)
	raw := model.JSON(map[string]any{"serviceId": serviceIdentity.ServiceID, "datasetGeneration": serviceIdentity.Generation, "clientRequestId": "old-epoch", "sessionId": a.SessionID, "targetId": taskID, "expectedVersion": 2, "integratorEpoch": 1, "payload": map[string]any{"limit": 0, "reason": "stale writer"}})
	if w := f.request("POST", "/api/v1/commands/collaborator.limit", string(raw), a.Token, ""); w.Code != 409 {
		t.Fatal("old epoch wrote", w.Code)
	}
	sessionRequest := model.JSON(map[string]any{"serviceId": serviceIdentity.ServiceID, "datasetGeneration": serviceIdentity.Generation, "clientRequestId": "new-session"})
	if w := f.request("POST", "/api/v1/sessions", string(sessionRequest), a.Token, ""); w.Code != 200 {
		t.Fatal("create explicit session", w.Code, w.Body.String())
	}
}

type authFixture struct {
	handler http.Handler
	ids     *identity.Service
	store   *store.Store
	cookie  *http.Cookie
	csrf    string
	core    *coordinator.Service
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	ctx := context.Background()
	st, e := store.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	ids := identity.New(st, nil)
	f := &authFixture{store: st, ids: ids}
	f.core = coordinator.New(st, nil, nil)
	f.handler = httpapi.New(f.core, ids, httpapi.Options{AllowedHost: "127.0.0.1:7337", BodyLimitBytes: 1024})
	path, e := ids.IssueOwnerBootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(path)
	m := regexp.MustCompile(`name="capability" value="([^"]+)"`).FindSubmatch(raw)
	if len(m) != 2 {
		t.Fatal("missing bootstrap")
	}
	request := httptest.NewRequest("POST", "http://127.0.0.1:7337/api/v1/setup/activate", strings.NewReader(url.Values{"capability": {string(m[1])}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "null")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, request)
	if w.Code != 303 {
		t.Fatalf("activate %d %s", w.Code, w.Body.String())
	}
	if location := w.Header().Get("Location"); location != "/" {
		t.Fatalf("activation redirects to %q, want the room home /", location)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("missing owner cookie")
	}
	f.cookie = cookies[0]
	if !f.cookie.HttpOnly || f.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("weak cookie")
	}
	csrf, e := ids.CSRF(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	f.csrf = csrf
	return f
}
func (f *authFixture) request(method, path, body, token, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:7337"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token == "owner" {
		r.AddCookie(f.cookie)
		r.Header.Set("X-CSRF-Token", f.csrf)
	} else if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func TestAgentCannotBecomeHuman(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	_, e := f.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(ctx, owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := f.ids.Enroll(ctx, cap, "agent", "worker")
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"/api/v1/commands/project.create", "/api/v1/commands/enrollment.enable", "/api/v1/commands/participant.revoke"} {
		w := f.request("POST", path, `{"actor":{"kind":"human"}}`, agent.Token, "")
		if w.Code != 403 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	if w := f.request("GET", "/api/v1/me", "", agent.Token, ""); strings.Contains(w.Body.String(), f.cookie.Value) || strings.Contains(w.Body.String(), f.csrf) {
		t.Fatal("owner credential exposed")
	}
}
func TestScopeRevokedAndOriginRejected(t *testing.T) {
	f := newAuthFixture(t)
	for _, origin := range []string{"https://foreign.example", "null"} {
		w := f.request("POST", "/api/v1/commands/project.create", "{}", "owner", origin)
		if w.Code != 403 {
			t.Fatal(origin, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "http://evil.example/identity", nil)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign Host")
	}
	w = f.request("POST", "/api/v1/commands/project.create", "{}", "", "")
	if w.Code != 401 {
		t.Fatal("missing token", w.Code)
	}
	w = f.request("POST", "/api/v1/commands/project.create", strings.Repeat("x", 2048), "owner", "http://127.0.0.1:7337")
	if w.Code != 413 {
		t.Fatal("body budget", w.Code)
	}
	r = httptest.NewRequest("POST", "http://127.0.0.1:7337/api/v1/commands/project.create", strings.NewReader("{}"))
	r.AddCookie(f.cookie)
	r.Header.Set("Origin", "http://127.0.0.1:7337")
	w = httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("missing CSRF", w.Code)
	}
}
func TestOwnerProjectCommandReplayAndStrictFields(t *testing.T) {
	f := newAuthFixture(t)
	id := f.store.Identity()
	body := map[string]any{"serviceId": id.ServiceID, "datasetGeneration": id.Generation, "clientRequestId": "create", "payload": map[string]any{"name": "Example", "repositories": []any{}}}
	raw, _ := json.Marshal(body)
	one := f.request("POST", "/api/v1/commands/project.create", string(raw), "owner", "http://127.0.0.1:7337")
	if one.Code != 200 {
		t.Fatal(one.Code, one.Body.String())
	}
	two := f.request("POST", "/api/v1/commands/project.create", string(raw), "owner", "http://127.0.0.1:7337")
	var first, second map[string]any
	_ = json.Unmarshal(one.Body.Bytes(), &first)
	_ = json.Unmarshal(two.Body.Bytes(), &second)
	if two.Code != 200 || first["committedEventId"] != second["committedEventId"] {
		t.Fatal("request replay", two.Code, two.Body.String())
	}
	body["actor"] = "human"
	raw, _ = json.Marshal(body)
	if w := f.request("POST", "/api/v1/commands/project.create", string(raw), "owner", "http://127.0.0.1:7337"); w.Code != 400 {
		t.Fatal("unknown field", w.Code)
	}
	delete(body, "actor")
	body["datasetGeneration"] = "old"
	raw, _ = json.Marshal(body)
	if w := f.request("POST", "/api/v1/commands/project.create", string(raw), "owner", "http://127.0.0.1:7337"); w.Code != 409 {
		t.Fatal("old generation replay", w.Code)
	}
	var count int
	if e := f.store.QueryRow(context.Background(), "SELECT count(*) FROM projects").Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
}
