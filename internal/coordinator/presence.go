package coordinator

import (
	"roooooom.local/roooooom/internal/model"
	"time"
)

func (s *Service) sessionView(tx rowReader, id model.ID) (map[string]any, error) {
	session, e := loadSession(tx, id)
	if e != nil {
		return nil, e
	}
	var last time.Time
	if raw, ok := session.Body["lastCheckpointAt"].(string); ok {
		last, _ = time.Parse(time.RFC3339Nano, raw)
	}
	s.observationMu.Lock()
	observed := s.observations[id]
	s.observationMu.Unlock()
	if observed.LastSeen.After(last) {
		last = observed.LastSeen
	}
	presence := "recent"
	if last.IsZero() || s.clock.Now().Sub(last) > 90*time.Second {
		presence = "unknown"
	}
	if session.Status == "ended" {
		presence = "ended"
	}
	result := session.Body
	result["id"] = id
	result["participantId"] = session.ParticipantID
	result["version"] = session.Version
	result["status"] = session.Status
	result["presence"] = presence
	result["lastSeenAt"] = last
	result["pendingWaits"] = observed.Waiting
	return result, nil
}
