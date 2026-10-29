package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"strconv"
)

func Connect(ctx context.Context, dataDir, handle string, project model.ID, reuseSession model.ID, transport http.RoundTripper) (Profile, error) {
	var e error
	dataDir, e = filepath.Abs(dataDir)
	if e != nil {
		return Profile{}, e
	}
	root, e := ProfileRoot(dataDir, handle)
	if e != nil {
		return Profile{}, e
	}
	p, e := LoadProfile(root)
	if e != nil {
		if !errors.Is(e, os.ErrNotExist) {
			return Profile{}, e
		}
		p = Profile{Root: root, Handle: handle, ProjectID: project, ServiceURL: "http://127.0.0.1:7337", Mode: "offline", Provisional: true}
	}
	if p.ProjectID == "" || project != "" && p.ProjectID != project {
		return p, model.Invalid("explicit matching projectId required")
	}
	discovery, e := privatefs.ReadFile(dataDir, "discovery.json")
	var info struct {
		ListenAddr      string `json:"listenAddr"`
		ServiceID       string `json:"serviceId"`
		Generation      string `json:"datasetGeneration"`
		ProtocolVersion int    `json:"protocolVersion"`
	}
	if e == nil {
		if json.Unmarshal(discovery, &info) != nil || info.ProtocolVersion != 1 {
			return p, model.Invalid("invalid private discovery")
		}
		p.ServiceURL = "http://" + info.ListenAddr
	} else if !errors.Is(e, os.ErrNotExist) {
		return p, e
	}
	c := New(p, transport)
	defer c.Close()
	if _, e = c.storage(); e != nil {
		return p, e
	}
	raw, e := c.request(ctx, "GET", "/identity", nil, false)
	if e != nil {
		var d *model.DomainError
		if errors.As(e, &d) && d.Code == "offline" {
			p.Mode = "offline"
			if p.SessionID == "" {
				p.SessionID = model.ID("local-" + string((model.RandomIDs{}).NewID()))
			}
			if e = SaveProfile(p); e != nil {
				return p, e
			}
			return p, nil
		}
		return p, e
	}
	var live struct {
		ServiceID       string `json:"serviceId"`
		Generation      string `json:"datasetGeneration"`
		ProtocolVersion int    `json:"protocolVersion"`
		Status          string
	}
	if json.Unmarshal(raw, &live) != nil || live.Status != "ok" || live.ProtocolVersion != 1 || info.ServiceID == "" || live.ServiceID != info.ServiceID || live.Generation != info.Generation {
		return p, model.Conflict("service-mismatch")
	}
	if p.ServiceID != "" && (p.ServiceID != live.ServiceID || p.Generation != live.Generation) {
		return p, model.Conflict("needs-reconcile")
	}
	p.ServiceID = live.ServiceID
	p.Generation = live.Generation
	c.Profile = p
	var credentials identity.Credentials
	if p.CredentialRef == "" {
		digest := sha256.Sum256([]byte(p.ProjectID))
		file := "enrollment/" + hex.EncodeToString(digest[:]) + ".json"
		raw, e = privatefs.ReadFile(dataDir, file)
		if e != nil {
			return p, &model.DomainError{Status: 403, Code: "enrollment-required", Message: "owner must enable this project before enrollment"}
		}
		var enrollment struct {
			ProjectID  model.ID
			Capability string
		}
		if json.Unmarshal(raw, &enrollment) != nil || enrollment.ProjectID != p.ProjectID || enrollment.Capability == "" {
			return p, model.Invalid("invalid project enrollment")
		}
		requestID, e := c.cache(ctx, "enrollment-request")
		if e != nil {
			return p, e
		}
		if requestID == "" {
			requestID = string((model.RandomIDs{}).NewID())
			if e = c.saveCache(ctx, "enrollment-request", requestID); e != nil {
				return p, e
			}
		}
		temporary := "credentials/enrollment.json"
		if e = privatefs.WriteAtomic(root, temporary, model.JSON(map[string]string{"token": enrollment.Capability})); e != nil {
			return p, e
		}
		c.Profile.CredentialRef = temporary
		raw, e = c.request(ctx, "POST", "/api/v1/enroll", model.JSON(map[string]any{"serviceId": p.ServiceID, "datasetGeneration": p.Generation, "clientRequestId": requestID, "name": handle}), true)
		if e != nil {
			return p, e
		}
	} else {
		response, e := c.query(ctx, "/api/v1/me")
		if e != nil {
			return p, e
		}
		var actor struct {
			ActorKind                string
			ParticipantID, SessionID model.ID
			ProjectIDs               []model.ID
		}
		if json.Unmarshal(response.Result, &actor) != nil || actor.ActorKind != "agent" || actor.ParticipantID != p.ParticipantID || actor.SessionID != p.SessionID {
			return p, model.Conflict("profile-actor-mismatch")
		}
		allowed := false
		for _, id := range actor.ProjectIDs {
			if id == p.ProjectID {
				allowed = true
			}
		}
		if !allowed {
			return p, model.Forbidden()
		}
		if reuseSession != "" {
			if reuseSession != p.SessionID {
				return p, model.Conflict("profile-session-mismatch")
			}
			p.Mode = "online"
			p.Provisional = false
			if e = SaveProfile(p); e != nil {
				return p, e
			}
			return p, nil
		}
		requestID, e := c.cache(ctx, "session-request")
		if e != nil {
			return p, e
		}
		if requestID == "" {
			requestID = string((model.RandomIDs{}).NewID())
			if e = c.saveCache(ctx, "session-request", requestID); e != nil {
				return p, e
			}
		}
		raw, e = c.request(ctx, "POST", "/api/v1/sessions", model.JSON(map[string]any{"serviceId": p.ServiceID, "datasetGeneration": p.Generation, "clientRequestId": requestID}), true)
		if e != nil {
			return p, e
		}
	}
	var response envelope
	if json.Unmarshal(raw, &response) != nil || json.Unmarshal(response.Result, &credentials) != nil || len(credentials.Token) != 43 || credentials.ParticipantID == "" || credentials.SessionID == "" {
		return p, model.Invalid("invalid enrollment response")
	}
	ref := "credentials/" + string(credentials.SessionID) + ".json"
	if e = privatefs.WriteAtomic(root, ref, model.JSON(credentials)); e != nil {
		return p, e
	}
	p.CredentialRef = ref
	p.ParticipantID = credentials.ParticipantID
	p.SessionID = credentials.SessionID
	p.Provisional = false
	p.Mode = "online"
	if e = SaveProfile(p); e != nil {
		return p, e
	}
	_ = c.saveCache(ctx, "session-request", "")
	return p, nil
}
func (c *Client) Query(ctx context.Context, q model.Query) (model.View, error) {
	values := url.Values{}
	if q.ProjectID != "" {
		values.Set("projectId", string(q.ProjectID))
	}
	if q.TargetID != "" {
		values.Set("targetId", string(q.TargetID))
	}
	if q.Cursor != "" {
		values.Set("cursor", q.Cursor)
	}
	if q.Limit > 0 {
		values.Set("limit", strconv.Itoa(q.Limit))
	}
	response, e := c.query(ctx, "/api/v1/queries/"+url.PathEscape(q.Kind)+"?"+values.Encode())
	if e != nil {
		return model.View{}, e
	}
	view := model.View{Data: response.Result, Cursor: response.Cursor, HasMore: response.HasMore}
	if e = c.saveCache(ctx, queryCacheKey(q), string(model.JSON(view))); e != nil {
		return view, e
	}
	return view, nil
}
