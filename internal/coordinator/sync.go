package coordinator

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"sort"
	"strings"
	"time"
)

type SyncPage struct {
	Events     []store.Event `json:"events"`
	Cursor     string        `json:"cursor"`
	HasMore    bool          `json:"hasMore"`
	Bytes      int           `json:"bytes"`
	ServerTime time.Time     `json:"serverTime"`
	Connection string        `json:"connection"`
}
type bookmark struct {
	ServiceID  string   `json:"serviceId"`
	Generation string   `json:"generation"`
	Scope      string   `json:"scope"`
	Seq        int64    `json:"seq"`
	AfterID    model.ID `json:"afterId,omitempty"`
}

func sortIDs(ids []model.ID) { sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] }) }
func scopeKey(a model.Actor, kind string) string {
	projects := append([]model.ID{}, a.ProjectIDs...)
	sortIDs(projects)
	raw := model.JSON(map[string]any{"actorId": a.ID, "actorKind": a.Kind, "projects": projects, "kind": kind})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *Service) encodeCursor(a model.Actor, kind string, seq int64, afterID model.ID) string {
	i := s.store.Identity()
	raw := model.JSON(bookmark{i.ServiceID, i.Generation, scopeKey(a, kind), seq, afterID})
	mac := hmac.New(sha256.New, s.store.CursorKey())
	mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) decodeCursor(a model.Actor, kind, cursor string) (bookmark, error) {
	if cursor == "" {
		return bookmark{}, nil
	}
	reset := func() (bookmark, error) {
		return bookmark{}, &model.DomainError{Status: 409, Code: "cursor_reset", Message: "full synchronization required"}
	}
	if len(cursor) > 4096 {
		return reset()
	}
	parts := strings.Split(cursor, ".")
	if len(parts) != 2 {
		return reset()
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return reset()
	}
	signature, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return reset()
	}
	mac := hmac.New(sha256.New, s.store.CursorKey())
	mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return reset()
	}
	var mark bookmark
	i := s.store.Identity()
	if json.Unmarshal(raw, &mark) != nil || mark.ServiceID != i.ServiceID || mark.Generation != i.Generation || mark.Scope != scopeKey(a, kind) || mark.Seq < 0 {
		return reset()
	}
	return mark, nil
}
func (s *Service) observe(a model.Actor, waitDelta int) {
	if a.SessionID == "" {
		return
	}
	s.observationMu.Lock()
	defer s.observationMu.Unlock()
	old := s.observations[a.SessionID]
	old.LastSeen = s.clock.Now()
	old.Waiting += waitDelta
	s.observations[a.SessionID] = old
}
func (s *Service) Sync(ctx context.Context, a model.Actor, cursor string) (SyncPage, error) {
	page := SyncPage{Events: []store.Event{}, ServerTime: s.clock.Now(), Connection: "online"}
	if a.Kind != "agent" && a.Kind != "human" {
		return page, model.Forbidden()
	}
	mark, e := s.decodeCursor(a, "sync", cursor)
	if e != nil {
		return page, e
	}
	e = s.store.Read(ctx, func(reader store.Reader) error {
		tx := readTx{ctx, reader}
		if a.Kind == "agent" {
			if len(a.ProjectIDs) == 0 {
				return model.Forbidden()
			}
			for _, project := range a.ProjectIDs {
				if e := authorize(tx, a, project); e != nil {
					return e
				}
			}
		}
		var max int64
		if e := tx.QueryRow("SELECT COALESCE(MAX(seq),0) FROM events").Scan(&max); e != nil {
			return e
		}
		if mark.Seq > max {
			return &model.DomainError{Status: 409, Code: "cursor_reset", Message: "full synchronization required"}
		}
		projects := string(model.JSON(a.ProjectIDs))
		rows, e := tx.Query(`SELECT e.seq,e.scope,e.body,e.received_at FROM events e WHERE e.seq>? AND (
   ?='human' OR e.scope IN(SELECT value FROM json_each(?))
   OR (e.scope LIKE 'message:%' AND EXISTS(SELECT 1 FROM messages m JOIN message_receipts r ON r.message_id=m.id WHERE m.id=substr(e.scope,9) AND r.participant_id=? AND m.project_id IN(SELECT value FROM json_each(?))))
   OR (e.scope=? AND json_extract(e.body,'$.receipt.projectId') IN(SELECT value FROM json_each(?)))
  ) ORDER BY e.seq LIMIT 101`, mark.Seq, a.Kind, projects, a.ParticipantID, projects, "receipt:"+string(a.ParticipantID), projects)
		if e != nil {
			return e
		}
		defer rows.Close()
		after := mark.Seq
		used := 0
		for rows.Next() {
			var event store.Event
			var raw string
			if e = rows.Scan(&event.Seq, &event.Scope, &raw, &event.ReceivedAt); e != nil {
				return e
			}
			event.Body = json.RawMessage(raw)
			size := len(model.JSON(event))
			if size > 60*1024 {
				var body struct {
					Kind       string
					ResourceID model.ID
				}
				if e = json.Unmarshal(event.Body, &body); e != nil {
					return e
				}
				event.Body = model.JSON(map[string]any{"kind": body.Kind, "resourceId": body.ResourceID, "messageRef": body.ResourceID})
				size = len(model.JSON(event))
			}
			if len(page.Events) >= 100 || used+size > 60*1024 {
				page.HasMore = true
				break
			}
			page.Events = append(page.Events, event)
			used += size
			after = event.Seq
		}
		if e = rows.Err(); e != nil {
			return e
		}
		if !page.HasMore {
			after = max
		}
		page.Cursor = s.encodeCursor(a, "sync", after, "")
		page.Bytes = len(model.JSON(page))
		return nil
	})
	if e == nil {
		s.observe(a, 0)
	}
	return page, e
}
func (s *Service) Wait(ctx context.Context, a model.Actor, cursor string, d time.Duration) (SyncPage, error) {
	if d == 0 {
		d = 30 * time.Second
	}
	if d < 0 || d > 60*time.Second {
		return SyncPage{}, model.Invalid("wait duration must be 0..60 seconds")
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	s.observe(a, 1)
	defer s.observe(a, -1)
	for {
		wake := s.store.Subscribe()
		if s.waitRegistered != nil {
			s.waitRegistered()
		}
		page, e := s.Sync(ctx, a, cursor)
		if e != nil {
			if ctx.Err() != nil && parent.Err() == nil {
				return SyncPage{Events: []store.Event{}, Cursor: cursor, ServerTime: s.clock.Now(), Connection: "online"}, nil
			}
			return page, e
		}
		if len(page.Events) > 0 || page.HasMore {
			return page, nil
		}
		cursor = page.Cursor
		if s.waitIdle != nil {
			s.waitIdle()
		}
		select {
		case <-wake:
			continue
		case <-ctx.Done():
			if parent.Err() != nil {
				return SyncPage{}, parent.Err()
			}
			return SyncPage{Events: []store.Event{}, Cursor: cursor, ServerTime: s.clock.Now(), Connection: "online"}, nil
		}
	}
}
