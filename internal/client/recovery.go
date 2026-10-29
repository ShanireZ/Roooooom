package client

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
)

// AdoptRecovery requires an explicitly selected private owner grant. It never
// changes participant identity, removes drafts or resumes any responsibility.
func AdoptRecovery(ctx context.Context, p Profile, file string, transport http.RoundTripper) (Profile, error) {
	if !filepath.IsAbs(file) {
		return p, model.Invalid("absolute private recovery file required")
	}
	raw, e := privatefs.ReadFile(filepath.Dir(file), filepath.Base(file))
	if e != nil {
		return p, e
	}
	var grant struct {
		ParticipantID model.ID `json:"participantId"`
		SessionID     model.ID `json:"sessionId"`
		ProjectID     model.ID `json:"projectId"`
		ServiceID     string   `json:"serviceId"`
		Generation    string   `json:"datasetGeneration"`
		Token         string   `json:"token"`
	}
	if model.Decode(raw, &grant) != nil || grant.ParticipantID != p.ParticipantID || grant.ProjectID != p.ProjectID || grant.ServiceID != p.ServiceID || grant.SessionID == "" || len(grant.Token) != 43 || grant.Generation == "" {
		return p, model.Conflict("recovery-profile-mismatch")
	}
	next := p
	next.Generation = grant.Generation
	next.SessionID = grant.SessionID
	next.Mode = "reconciling"
	next.Provisional = false
	c := New(next, transport)
	defer c.Close()
	// Verify the live identity before reading/transmitting a bearer credential.
	if e = c.Verify(ctx); e != nil {
		return p, e
	}
	next.CredentialRef = "credentials/" + string((model.RandomIDs{}).NewID()) + ".json"
	if e = privatefs.WriteAtomic(next.Root, next.CredentialRef, model.JSON(map[string]any{"participantId": grant.ParticipantID, "sessionId": grant.SessionID, "token": grant.Token})); e != nil {
		return p, e
	}
	c.Profile = next
	me, e := c.query(ctx, "/api/v1/me")
	if e != nil {
		return p, e
	}
	var actor struct {
		ParticipantID, SessionID model.ID
		ActorKind                string
		ProjectIDs               []model.ID
	}
	if json.Unmarshal(me.Result, &actor) != nil || actor.ActorKind != "agent" || actor.ParticipantID != p.ParticipantID || actor.SessionID != next.SessionID || len(actor.ProjectIDs) != 1 || actor.ProjectIDs[0] != p.ProjectID {
		return p, model.Forbidden()
	}
	if e = SaveProfile(next); e != nil {
		return p, e
	}
	if e = c.saveCache(ctx, "cursor", ""); e != nil {
		return next, e
	}
	if e = c.saveCache(ctx, "session-request", ""); e != nil {
		return next, e
	}
	if e = c.snapshotCurrent(ctx); e != nil {
		return next, e
	}
	return next, nil
}
