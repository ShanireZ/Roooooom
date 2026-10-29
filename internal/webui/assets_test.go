package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedAppCannotReadProjectFiles(t *testing.T) {
	handler := Handler()
	for _, path := range []string{"/AGENTS.md", "/.git/config", "/credentials/owner.json", "/assets/../PLAN.md", "/assets/%2e%2e/PLAN.md", "/assets/missing.js"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:7337"+path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/", "/setup", "/projects", "/tasks/synthetic-task", "/decisions", "/settings"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:7337"+path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "<div id=\"root\">") {
			t.Fatalf("SPA deep link %s: %d", path, w.Code)
		}
	}
}
