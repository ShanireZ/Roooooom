package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"roooooom.local/roooooom/internal/model"
	"time"
)

func (a *api) events(w http.ResponseWriter, r *http.Request, actor model.Actor) {
	cursor := r.URL.Query().Get("cursor")
	if last := r.Header.Get("Last-Event-ID"); last != "" {
		cursor = last
	}
	page, e := a.core.Sync(r.Context(), actor, cursor)
	if e != nil {
		fail(w, e)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, &model.DomainError{Status: 503, Code: "stream-unavailable", Message: "streaming not available"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	if _, e = fmt.Fprint(w, ": connected\n\n"); e != nil {
		return
	}
	flusher.Flush()
	for {
		// A long-lived response must revalidate its credential before releasing
		// each batch, including after a wait has been woken by revocation.
		current, _, _, authErr := a.authenticate(r)
		if authErr != nil || current.ID != actor.ID || current.Kind != actor.Kind || current.SessionID != actor.SessionID {
			return
		}
		raw, e := json.Marshal(page)
		if e != nil {
			return
		}
		if _, e = fmt.Fprintf(w, "id: %s\nevent: workspace\ndata: %s\n\n", page.Cursor, raw); e != nil {
			return
		}
		flusher.Flush()
		cursor = page.Cursor
		if page.HasMore {
			page, e = a.core.Sync(r.Context(), actor, cursor)
		} else {
			page, e = a.core.Wait(r.Context(), actor, cursor, 30*time.Second)
		}
		if e != nil {
			return
		}
	}
}
