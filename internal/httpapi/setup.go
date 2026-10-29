package httpapi

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/i18n"
	"roooooom.local/roooooom/internal/model"
	"strings"
)

func (a *api) activate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		fail(w, model.NotFound())
		return
	}
	if r.ContentLength > a.opts.BodyLimitBytes {
		fail(w, &model.DomainError{Status: 413, Code: "body-too-large", Message: "body budget exceeded"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.opts.BodyLimitBytes)
	if e := r.ParseForm(); e != nil {
		fail(w, model.Invalid("invalid activation form"))
		return
	}
	if len(r.PostForm) != 1 || len(r.PostForm["capability"]) != 1 {
		fail(w, model.Invalid("activation capability required"))
		return
	}
	token, _, e := a.identities.Activate(r.Context(), r.PostForm.Get("capability"))
	if e != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			owner, _ := a.core.OwnerLanguage(r.Context())
			accepted, _ := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
			l := i18n.First(i18n.Locale(owner), accepted)
			t := func(k i18n.Key) string { return html.EscapeString(i18n.T(l, k)) }
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(model.Status(e))
			fmt.Fprint(w, `<!doctype html><html lang="`+string(l)+`"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`+t(i18n.ExpiredTitle)+`</title><link rel="stylesheet" href="/assets/server-pages.css"><main><strong class="mark">ROOOOOOM</strong><h1>`+t(i18n.ExpiredHeading)+`</h1><p>`+t(i18n.ExpiredBody)+`</p><p>`+t(i18n.ExpiredNote)+`</p></main></html>`)
			return
		}
		fail(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "roooooom_owner", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *api) connectionInfo(w http.ResponseWriter, r *http.Request, actor model.Actor) {
	if actor.Kind != "human" {
		fail(w, model.Forbidden())
		return
	}
	executable, e := os.Executable()
	if e != nil {
		fail(w, model.Invalid("local client path unavailable"))
		return
	}
	a.respond(w, map[string]any{"dataDir": a.identities.Root(), "clientPath": executable, "skillPath": filepath.Join(filepath.Dir(executable), "skills", "roooooom", "SKILL.md"), "serviceURL": a.origin})
}
