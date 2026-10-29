package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
)

//go:embed dist
var content embed.FS
var taskRoute = regexp.MustCompile("^/tasks/[A-Za-z0-9_-]{1,128}$")
var setupRoute = regexp.MustCompile("^/setup/[A-Za-z0-9_-]{1,128}$")
var requestRoute = regexp.MustCompile("^/requests/[A-Za-z0-9_-]{1,128}$")

// roomRoute mirrors web/src/shell/routes.ts; drawer state lives in the path.
// "new" and "settings" are reserved after /rooms/; system settings live at /settings.
var roomRoute = regexp.MustCompile("^/rooms/[A-Za-z0-9_-]{1,128}(/(tasks|requests|decisions|acceptance)/[A-Za-z0-9_-]{1,128}(/(overview|work|evidence|discussion|history|manage))?|/archive|/settings(/(door|members|integrations))?)?$")

func Route(p string) bool {
	return p == "/" || p == "/setup" || p == "/projects" || p == "/decisions" || p == "/settings" || taskRoute.MatchString(p) || setupRoute.MatchString(p) || requestRoute.MatchString(p) || (roomRoute.MatchString(p) && p != "/rooms/settings") || strings.HasPrefix(p, "/assets/")
}
func Handler() http.Handler {
	files, _ := fs.Sub(content, "dist")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !Route(r.URL.Path) || r.URL.RawQuery != "" || strings.Contains(r.URL.EscapedPath(), "%") || path.Clean(r.URL.Path) != r.URL.Path {
			http.NotFound(w, r)
			return
		}
		name := "index.html"
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			name = strings.TrimPrefix(r.URL.Path, "/")
			if strings.Contains(name, "..") || strings.ContainsAny(name, "\\\x00") {
				http.NotFound(w, r)
				return
			}
		}
		bytes, e := fs.ReadFile(files, name)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		switch path.Ext(name) {
		case ".html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case ".js":
			w.Header().Set("Content-Type", "application/javascript")
		case ".css":
			w.Header().Set("Content-Type", "text/css")
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Write(bytes)
	})
}
