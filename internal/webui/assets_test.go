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

func TestEmbeddedServerPageStylesheet(t *testing.T) {
	r := httptest.NewRequest("GET", "http://127.0.0.1:7337/assets/server-pages.css", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") || !strings.Contains(w.Body.String(), "--paper:#f6f3ea") {
		t.Fatalf("server page stylesheet: %d %q", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestRouteAllowsRooms(t *testing.T) {
	for _, p := range []string{"/rooms/new", "/rooms/settings", "/rooms/p1", "/rooms/p1/tasks/t1", "/rooms/p1/tasks/t1/work", "/rooms/p1/requests/r1", "/rooms/p1/decisions/d1", "/rooms/p1/acceptance/t1", "/rooms/p1/archive", "/rooms/p1/settings", "/rooms/p1/settings/members"} {
		if !Route(p) {
			t.Errorf("room route %s rejected", p)
		}
	}
	for _, p := range []string{"/rooms", "/rooms/", "/rooms/p1/tasks", "/rooms/p1/tasks/t1/bogus", "/rooms/p1/../x", "/rooms/p1/settings/other", "/rooms/a b"} {
		if Route(p) {
			t.Errorf("malformed room route %s accepted", p)
		}
	}
	r := httptest.NewRequest("GET", "http://127.0.0.1:7337/rooms/p1?x=1", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("room route with query string: %d", w.Code)
	}
}
