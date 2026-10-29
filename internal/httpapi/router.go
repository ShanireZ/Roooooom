package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"roooooom.local/roooooom/internal/control"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/webui"
	"strconv"
	"strings"
)

type Options struct {
	Control        *control.Server
	AllowedHost    string
	Clock          model.Clock
	BodyLimitBytes int64
}
type commandBody struct {
	model.Mutation
	TargetID model.ID        `json:"targetId,omitempty"`
	Payload  json.RawMessage `json:"payload"`
}
type api struct {
	core       *coordinator.Service
	identities *identity.Service
	opts       Options
	origin     string
}

func New(core *coordinator.Service, ids *identity.Service, opts Options) http.Handler {
	host, _, e := net.SplitHostPort(opts.AllowedHost)
	if e != nil || host != "127.0.0.1" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "invalid listener configuration", 503) })
	}
	if opts.Clock == nil {
		opts.Clock = model.RealClock{}
	}
	if opts.BodyLimitBytes <= 0 {
		opts.BodyLimitBytes = 1024 * 1024
	}
	a := &api{core, ids, opts, "http://" + opts.AllowedHost}
	ids.SetOrigin(a.origin)
	return a
}
func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
	if r.Host != a.opts.AllowedHost {
		fail(w, model.Forbidden())
		return
	}
	if r.URL.RawQuery != "" {
		queryRoute := strings.HasPrefix(r.URL.Path, "/api/v1/queries/")
		streamRoute := r.URL.Path == "/api/v1/sync" || r.URL.Path == "/api/v1/wait" || r.URL.Path == "/api/v1/events"
		if r.Method != "GET" || (!queryRoute && !streamRoute) {
			fail(w, model.Forbidden())
			return
		}
		values, e := url.ParseQuery(r.URL.RawQuery)
		if e != nil {
			fail(w, model.Invalid("invalid query"))
			return
		}
		for key, value := range values {
			if len(value) != 1 || len(value[0]) > 4096 {
				fail(w, model.Invalid("invalid query"))
				return
			}
			if streamRoute {
				if key != "cursor" && (key != "timeoutSeconds" || r.URL.Path != "/api/v1/wait") {
					fail(w, model.Invalid("unknown query field"))
					return
				}
				continue
			}
			switch key {
			case "projectId", "targetId", "limit", "cursor":
			default:
				fail(w, model.Invalid("unknown query field"))
				return
			}
		}
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != a.origin && !(r.URL.Path == "/api/v1/setup/activate" && origin == "null") {
		fail(w, model.Forbidden())
		return
	}
	if r.URL.Path == "/health" || r.URL.Path == "/identity" {
		id := a.identities.Identity()
		Health(id.ServiceID, id.Generation).ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/api/v1/setup/activate" {
		a.activate(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/control/") && a.opts.Control != nil {
		a.opts.Control.ServeHTTP(w, r)
		return
	}
	if r.Method == "GET" && webui.Route(r.URL.Path) {
		webui.Handler().ServeHTTP(w, r)
		return
	}
	actor, token, bearer, e := a.authenticate(r)
	if e != nil {
		fail(w, e)
		return
	}
	if actor.Kind == "enrollment" && r.URL.Path != "/api/v1/enroll" {
		fail(w, model.Forbidden())
		return
	}
	if r.Method == "GET" {
		switch r.URL.Path {
		case "/api/v1/setup/connection":
			a.connectionInfo(w, r, actor)
			return
		case "/api/v1/sync":
			page, e := a.core.Sync(r.Context(), actor, r.URL.Query().Get("cursor"))
			if e != nil {
				fail(w, e)
				return
			}
			a.respond(w, page)
			return
		case "/api/v1/wait":
			a.wait(w, r, actor)
			return
		case "/api/v1/events":
			a.events(w, r, actor)
			return
		}
	}
	if r.Method == "POST" {
		if actor.Kind == "human" {
			csrf, e := a.identities.CSRF(r.Context(), token)
			if bearer || r.Header.Get("Origin") != a.origin || e != nil || r.Header.Get("X-CSRF-Token") != csrf {
				fail(w, model.Forbidden())
				return
			}
		}
		if r.ContentLength > a.opts.BodyLimitBytes {
			fail(w, &model.DomainError{Code: "body-too-large", Status: 413, Message: "body budget exceeded"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, a.opts.BodyLimitBytes)
	}
	if r.URL.Path == "/api/v1/me" && r.Method == "GET" {
		result := map[string]any{"actorId": actor.ID, "actorKind": actor.Kind, "participantId": actor.ParticipantID, "sessionId": actor.SessionID, "projectIds": actor.ProjectIDs}
		if actor.Kind == "human" && !bearer {
			csrf, e := a.identities.CSRF(r.Context(), token)
			if e != nil {
				fail(w, e)
				return
			}
			result["csrf"] = csrf
		}
		a.respond(w, result)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/queries/") && r.Method == "GET" {
		values := r.URL.Query()
		limit := 0
		if raw := values.Get("limit"); raw != "" {
			limit, e = strconv.Atoi(raw)
			if e != nil {
				fail(w, model.Invalid("invalid limit"))
				return
			}
		}
		view, e := a.core.Read(r.Context(), actor, model.Query{Kind: strings.TrimPrefix(r.URL.Path, "/api/v1/queries/"), ProjectID: model.ID(values.Get("projectId")), TargetID: model.ID(values.Get("targetId")), Cursor: values.Get("cursor"), Limit: limit})
		if e != nil {
			fail(w, e)
			return
		}
		a.respondFor(w, r, actor, view)
		return
	}
	if r.URL.Path == "/api/v1/sessions" && r.Method == "POST" {
		if actor.Kind != "agent" || !bearer {
			fail(w, model.Forbidden())
			return
		}
		var body struct {
			ServiceID       string `json:"serviceId"`
			Generation      string `json:"datasetGeneration"`
			ClientRequestID string `json:"clientRequestId"`
		}
		if e = decode(w, r, &body); e != nil {
			fail(w, e)
			return
		}
		id := a.identities.Identity()
		if body.ServiceID == "" || body.Generation == "" {
			fail(w, model.Invalid("identity and generation required"))
			return
		}
		if body.ServiceID != id.ServiceID || body.Generation != id.Generation {
			fail(w, model.Conflict("needs-reconcile"))
			return
		}
		credentials, e := a.identities.NewSession(r.Context(), actor, body.ClientRequestID)
		if e != nil {
			fail(w, e)
			return
		}
		a.respond(w, credentials)
		return
	}
	if r.URL.Path == "/api/v1/enroll" && r.Method == "POST" {
		if actor.Kind != "enrollment" || !bearer {
			fail(w, model.Forbidden())
			return
		}
		var b struct {
			ServiceID       string `json:"serviceId"`
			Generation      string `json:"datasetGeneration"`
			ClientRequestID string `json:"clientRequestId"`
			Name            string `json:"name"`
		}
		if e = decode(w, r, &b); e != nil {
			fail(w, e)
			return
		}
		id := a.identities.Identity()
		if b.ServiceID == "" || b.Generation == "" {
			fail(w, model.Invalid("identity and generation required"))
			return
		}
		if b.ServiceID != id.ServiceID || b.Generation != id.Generation {
			fail(w, model.Conflict("needs-reconcile"))
			return
		}
		credentials, e := a.identities.Enroll(r.Context(), token, b.ClientRequestID, b.Name)
		if e != nil {
			fail(w, e)
			return
		}
		a.respond(w, credentials)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/commands/") && r.Method == "POST" {
		kind := strings.TrimPrefix(r.URL.Path, "/api/v1/commands/")
		if ownerOnly(kind) && actor.Kind != "human" {
			fail(w, model.Forbidden())
			return
		}
		if actor.Kind != "agent" && actor.Kind != "human" {
			fail(w, model.Forbidden())
			return
		}
		var b commandBody
		if e = decode(w, r, &b); e != nil {
			fail(w, e)
			return
		}
		c := model.Command{Kind: kind, TargetID: b.TargetID, Meta: b.Mutation, Payload: b.Payload}
		var receipt model.Receipt
		if kind == "enrollment.enable" || kind == "participant.revoke" || kind == "participant.recover" {
			receipt, e = a.identities.Control(r.Context(), actor, c)
		} else if kind == "backup.create" || kind == "backup.export" {
			receipt, e = a.core.Maintenance(r.Context(), actor, c)
		} else if strings.HasPrefix(kind, "binding.") {
			receipt, e = a.core.Outbound(r.Context(), actor, c)
		} else {
			receipt, e = a.core.Apply(r.Context(), actor, c)
		}
		if e != nil {
			fail(w, e)
			return
		}
		a.respondFor(w, r, actor, receipt)
		return
	}
	fail(w, model.NotFound())
}
func ownerOnly(kind string) bool {
	if kind == "request.create" || kind == "request.cancel" || kind == "request.assign" {
		return true
	}
	return strings.HasPrefix(kind, "project.") || strings.HasPrefix(kind, "enrollment.") || strings.HasPrefix(kind, "participant.") || strings.HasPrefix(kind, "binding.") || strings.HasPrefix(kind, "backup.") || kind == "decision.resolve" || kind == "decision.supersede" || kind == "decision.withdraw" || kind == "service.stop"
}
func (a *api) authenticate(r *http.Request) (model.Actor, string, bool, error) {
	token := ""
	bearer := false
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimPrefix(auth, "Bearer ")
		bearer = true
	} else if c, e := r.Cookie("roooooom_owner"); e == nil {
		token = c.Value
	}
	actor, e := a.identities.Authenticate(r.Context(), token)
	if e != nil {
		return actor, "", bearer, e
	}
	if actor.Kind == "human" && bearer {
		return model.Actor{}, "", true, model.Forbidden()
	}
	if actor.Kind != "human" && !bearer {
		return model.Actor{}, "", false, model.Forbidden()
	}
	return actor, token, bearer, nil
}
func decode(w http.ResponseWriter, r *http.Request, target any) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return model.Invalid("application/json required")
	}
	raw, e := io.ReadAll(r.Body)
	if e != nil {
		var big *http.MaxBytesError
		if errors.As(e, &big) {
			return &model.DomainError{Status: 413, Code: "body-too-large", Message: "body budget exceeded"}
		}
		return model.Invalid("body unavailable")
	}
	return model.Decode(raw, target)
}
func (a *api) respond(w http.ResponseWriter, result any, unread ...[]any) {
	envelope := map[string]any{"requestId": "", "result": result, "objectVersions": map[string]int64{}, "committedEventId": 0, "unread": []any{}, "cursor": "", "hasMore": false, "serverTime": a.opts.Clock.Now(), "connection": "online"}
	if receipt, ok := result.(model.Receipt); ok {
		envelope["requestId"] = receipt.RequestID
		envelope["result"] = receipt.Result
		envelope["objectVersions"] = map[model.ID]int64{receipt.ResourceID: receipt.Version}
		envelope["committedEventId"] = receipt.EventID
	}
	if view, ok := result.(model.View); ok {
		envelope["result"] = view.Data
		envelope["cursor"] = view.Cursor
		envelope["hasMore"] = view.HasMore
	}
	if page, ok := result.(coordinator.SyncPage); ok {
		envelope["cursor"] = page.Cursor
		envelope["hasMore"] = page.HasMore
	}
	if len(unread) > 0 {
		envelope["unread"] = unread[0]
	}
	write(w, 200, envelope)
}
func (a *api) respondFor(w http.ResponseWriter, r *http.Request, actor model.Actor, result any) {
	unread, e := a.core.UnreadSummaries(r.Context(), actor)
	if e != nil {
		unread = []any{}
	}
	a.respond(w, result, unread)
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, e error) {
	var domain *model.DomainError
	if !errors.As(e, &domain) {
		domain = &model.DomainError{Status: 500, Code: "internal", Message: "operation failed"}
	}
	if domain.Status == 429 || domain.Code == "storage-busy" {
		w.Header().Set("Retry-After", "1")
	}
	write(w, domain.Status, map[string]any{"error": map[string]string{"code": domain.Code, "message": domain.Message}})
}
