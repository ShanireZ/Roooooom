package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/model"
	"strings"
	"testing"
)

func TestGuidedConnectionInfoIsOwnerOnly(t *testing.T) {
	f := newAuthFixture(t)
	owner, e := f.ids.Authenticate(t.Context(), f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	identity := f.store.Identity()
	project, e := f.core.Apply(t.Context(), owner, model.Command{Kind: "project.create", Payload: model.JSON(coordinator.ProjectPayload{Name: "join", Repositories: []coordinator.RepositoryPayload{}}), Meta: model.Mutation{ServiceID: identity.ServiceID, Generation: identity.Generation, ClientRequestID: "setup-project"}})
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(t.Context(), owner, project.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.ids.Enroll(t.Context(), cap, "join", "join")
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		token  string
		status int
	}{{"", 401}, {credentials.Token, 403}, {"owner", 200}} {
		response := f.request("GET", "/api/v1/setup/connection", "", test.token, "")
		if response.Code != test.status {
			t.Fatalf("connection info scope got %d want %d", response.Code, test.status)
		}
		if test.token == "owner" {
			var value struct {
				Result struct {
					DataDir    string `json:"dataDir"`
					ClientPath string `json:"clientPath"`
					SkillPath  string `json:"skillPath"`
					ServiceURL string `json:"serviceURL"`
				}
			}
			if e = json.Unmarshal(response.Body.Bytes(), &value); e != nil {
				t.Fatal(e)
			}
			if value.Result.DataDir != filepath.Dir(f.store.Path) || !filepath.IsAbs(value.Result.ClientPath) || !strings.HasSuffix(filepath.ToSlash(value.Result.SkillPath), "/skills/roooooom/SKILL.md") || value.Result.ServiceURL != "http://127.0.0.1:7337" {
				t.Fatal("incomplete local join metadata", value)
			}
			if strings.Contains(response.Body.String(), f.cookie.Value) || strings.Contains(response.Body.String(), cap) || strings.Contains(response.Body.String(), credentials.Token) {
				t.Fatal("join instructions leaked a capability")
			}
		}
	}
}
func TestGuidedExpiredActivationHasRecoveryInstructions(t *testing.T) {
	f := newAuthFixture(t)
	raw, e := os.ReadFile(filepath.Join(filepath.Dir(f.store.Path), "activation.html"))
	if e != nil {
		t.Fatal(e)
	}
	capability := regexp.MustCompile(`name="capability" value="([^"]+)"`).FindSubmatch(raw)
	if len(capability) != 2 {
		t.Fatal("missing fixture capability")
	}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7337/api/v1/setup/activate", strings.NewReader(url.Values{"capability": {string(capability[1])}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "null")
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != 409 || !strings.Contains(response.Body.String(), "重新啟用") || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
		t.Fatal("expired activation is not actionable", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 || strings.Contains(response.Body.String(), string(capability[1])) {
		t.Fatal("expired page renewed or exposed authority")
	}
}
