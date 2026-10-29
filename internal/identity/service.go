package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type Service struct {
	store  *store.Store
	clock  model.Clock
	origin string
}
type Credentials struct {
	ParticipantID model.ID `json:"participantId"`
	SessionID     model.ID `json:"sessionId"`
	Token         string   `json:"token"`
}
type tokenBody struct {
	ParticipantID model.ID  `json:"participantId,omitempty"`
	CSRF          string    `json:"csrf,omitempty"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

func New(st *store.Store, clock model.Clock) *Service {
	if clock == nil {
		clock = model.RealClock{}
	}
	return &Service{store: st, clock: clock}
}
func (s *Service) SetOrigin(origin string)  { s.origin = origin }
func (s *Service) Identity() store.Identity { return s.store.Identity() }
func (s *Service) Root() string             { return filepath.Dir(s.store.Path) }
func random() string                        { return string((model.RandomIDs{}).NewID()) }
func hash(secret string) string             { h := sha256.Sum256([]byte(secret)); return hex.EncodeToString(h[:]) }
func marshal(value any) string {
	b, e := json.Marshal(value)
	if e != nil {
		panic(e)
	}
	return string(b)
}
func denied(status int, code string) error {
	return &model.DomainError{Status: status, Code: code, Message: "access denied"}
}
func insertToken(tx *store.Tx, token, actor, kind string, session, project any, body tokenBody) error {
	_, e := tx.Exec("INSERT INTO tokens(hash,actor_id,kind,session_id,project_id,body) VALUES(?,?,?,?,?,?)", hash(token), actor, kind, session, project, marshal(body))
	return e
}
func (s *Service) Authenticate(ctx context.Context, token string) (model.Actor, error) {
	if len(token) != 43 {
		return model.Actor{}, denied(401, "unauthorized")
	}
	var actor model.Actor
	var session, project *string
	var raw string
	e := s.store.QueryRow(ctx, "SELECT actor_id,kind,session_id,project_id,body FROM tokens WHERE hash=? AND revoked=0", hash(token)).Scan(&actor.ID, &actor.Kind, &session, &project, &raw)
	var body tokenBody
	if e != nil || json.Unmarshal([]byte(raw), &body) != nil || (!body.ExpiresAt.IsZero() && !s.clock.Now().Before(body.ExpiresAt)) {
		return model.Actor{}, denied(401, "unauthorized")
	}
	actor.ParticipantID = body.ParticipantID
	if session != nil {
		actor.SessionID = model.ID(*session)
	}
	if project != nil {
		actor.ProjectIDs = []model.ID{model.ID(*project)}
		if actor.Kind == "agent" {
			var n int
			e = s.store.QueryRow(ctx, "SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", actor.ParticipantID, *project).Scan(&n)
			if e != nil || n != 1 {
				return model.Actor{}, denied(401, "unauthorized")
			}
		}
	}
	return actor, nil
}
func (s *Service) CSRF(ctx context.Context, token string) (string, error) {
	a, e := s.Authenticate(ctx, token)
	if e != nil {
		return "", e
	}
	if a.Kind != "human" {
		return "", denied(403, "forbidden")
	}
	var raw string
	e = s.store.QueryRow(ctx, "SELECT body FROM tokens WHERE hash=?", hash(token)).Scan(&raw)
	var b tokenBody
	if e != nil || json.Unmarshal([]byte(raw), &b) != nil {
		return "", denied(401, "unauthorized")
	}
	return b.CSRF, nil
}
