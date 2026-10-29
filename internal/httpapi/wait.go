package httpapi

import (
	"net/http"
	"roooooom.local/roooooom/internal/model"
	"strconv"
	"time"
)

func (a *api) wait(w http.ResponseWriter, r *http.Request, actor model.Actor) {
	duration := 30 * time.Second
	if value := r.URL.Query().Get("timeoutSeconds"); value != "" {
		seconds, e := strconv.Atoi(value)
		if e != nil || seconds < 1 || seconds > 60 {
			fail(w, model.Invalid("timeoutSeconds must be 1..60"))
			return
		}
		duration = time.Duration(seconds) * time.Second
	}
	page, e := a.core.Wait(r.Context(), actor, r.URL.Query().Get("cursor"), duration)
	if e != nil {
		if r.Context().Err() == nil {
			fail(w, e)
		}
		return
	}
	a.respond(w, page)
}
