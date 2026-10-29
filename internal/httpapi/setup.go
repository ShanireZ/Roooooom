package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(model.Status(e))
			fmt.Fprint(w, `<!doctype html><html lang="zh-Hant"><meta charset="utf-8"><title>重新啟用 Roooooom</title><h1>啟用頁已過期或已使用</h1><p>請從 Roooooom 托盤選擇「重新啟用」，再點「開始使用」。沒有托盤時，可使用本產品啟動器的重新啟用入口。</p><p>這不會刪除專案或協作紀錄。</p></html>`)
			return
		}
		fail(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "roooooom_owner", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 86400})
	http.Redirect(w, r, "/setup", http.StatusSeeOther)
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
