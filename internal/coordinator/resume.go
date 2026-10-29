package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"sort"
	"strings"
	"time"
)

type checkpointPayload struct {
	Summary                string     `json:"summary"`
	HandoffTargetSessionID model.ID   `json:"handoffTargetSessionId,omitempty"`
	ResponsibilityIDs      []model.ID `json:"responsibilityIds,omitempty"`
}
type resumePayload struct {
	ExpectedSourceSessionID model.ID   `json:"expectedSourceSessionId"`
	SourceVersion           int64      `json:"sourceVersion"`
	ResponsibilityIDs       []model.ID `json:"responsibilityIds"`
	HandoffCapabilityRef    string     `json:"handoffCapabilityRef,omitempty"`
	Force                   bool       `json:"force,omitempty"`
	Reason                  string     `json:"reason,omitempty"`
}
type sessionRecord struct {
	ID, ParticipantID model.ID
	Version           int64
	Status            string
	Body              map[string]any
}

func sessionProject(tx rowReader, id model.ID) (model.ID, error) {
	var project model.ID
	e := tx.QueryRow("SELECT project_id FROM tokens WHERE session_id=? AND kind='agent' LIMIT 1", id).Scan(&project)
	if e != nil {
		return "", model.NotFound()
	}
	return project, nil
}

func loadSession(tx rowReader, id model.ID) (sessionRecord, error) {
	s := sessionRecord{ID: id}
	var raw string
	e := tx.QueryRow("SELECT participant_id,version,status,body FROM sessions WHERE id=?", id).Scan(&s.ParticipantID, &s.Version, &s.Status, &raw)
	if e != nil {
		return s, model.NotFound()
	}
	e = json.Unmarshal([]byte(raw), &s.Body)
	return s, e
}
func saveSession(tx *store.Tx, s *sessionRecord) error {
	result, e := tx.Exec("UPDATE sessions SET version=version+1,status=?,body=? WHERE id=? AND version=?", s.Status, string(model.JSON(s.Body)), s.ID, s.Version)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return model.Conflict("stale-session-version")
	}
	s.Version++
	return nil
}
func normalizePayload(p any) {
	switch p := p.(type) {
	case *resumePayload:
		sort.Slice(p.ResponsibilityIDs, func(i, j int) bool { return p.ResponsibilityIDs[i] < p.ResponsibilityIDs[j] })
	case *checkpointPayload:
		sort.Slice(p.ResponsibilityIDs, func(i, j int) bool { return p.ResponsibilityIDs[i] < p.ResponsibilityIDs[j] })
	case *handoffPayload:
		for i := range p.Dispositions {
			d := &p.Dispositions[i]
			if d.TargetSessionID == "" {
				if d.Action == "transfer" {
					d.TargetSessionID = p.ToSessionID
				}
				if d.Action == "continue-worker" {
					d.TargetSessionID = p.FromSessionID
				}
			}
		}
		sort.Slice(p.Dispositions, func(i, j int) bool { return p.Dispositions[i].AssignmentID < p.Dispositions[j].AssignmentID })
	}
}
func digestSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

type resumeCapability struct {
	SourceSessionID   model.ID   `json:"sourceSessionId"`
	SourceVersion     int64      `json:"sourceVersion"`
	TargetSessionID   model.ID   `json:"targetSessionId"`
	ResponsibilityIDs []model.ID `json:"responsibilityIds"`
	ExpiresAt         time.Time  `json:"expiresAt"`
}

func (s *Service) checkpoint(tx *store.Tx, a model.Actor, c model.Command, p checkpointPayload) (model.ID, int64, []byte, error) {
	if a.Kind != "agent" || c.TargetID != a.SessionID {
		return "", 0, nil, model.Forbidden()
	}
	session, e := loadSession(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, session.Version); e != nil {
		return "", 0, nil, e
	}
	if p.Summary == "" || len(p.Summary) > 8192 {
		return "", 0, nil, model.Invalid("bounded checkpoint summary required")
	}
	session.Body["summary"] = p.Summary
	session.Body["lastCheckpointAt"] = s.clock.Now()
	if c.Kind == "session.end" {
		session.Status = "ended"
	}
	if e = saveSession(tx, &session); e != nil {
		return "", 0, nil, e
	}
	result := map[string]any{"id": session.ID, "version": session.Version, "status": session.Status}
	if p.HandoffTargetSessionID != "" {
		target, e := loadSession(tx, p.HandoffTargetSessionID)
		if e != nil {
			return "", 0, nil, e
		}
		sourceProject, e := sessionProject(tx, session.ID)
		if e != nil {
			return "", 0, nil, e
		}
		targetProject, e := sessionProject(tx, target.ID)
		if e != nil {
			return "", 0, nil, e
		}
		if sourceProject != targetProject {
			return "", 0, nil, model.Forbidden()
		}
		if target.ParticipantID != a.ParticipantID || target.ID == session.ID || target.Status != "active" || len(p.ResponsibilityIDs) == 0 || len(p.ResponsibilityIDs) > 2000 {
			return "", 0, nil, model.Invalid("explicit same-participant target and responsibilities required")
		}
		for i, id := range p.ResponsibilityIDs {
			if i > 0 && id == p.ResponsibilityIDs[i-1] {
				return "", 0, nil, model.Invalid("duplicate responsibility")
			}
			if e = ownsResponsibility(tx, id, session.ID, a.ParticipantID); e != nil {
				return "", 0, nil, e
			}
		}
		cap := string(s.ids.NewID())
		ref := "resume-caps/" + string(s.ids.NewID()) + ".json"
		body := resumeCapability{session.ID, session.Version, target.ID, p.ResponsibilityIDs, s.clock.Now().Add(5 * time.Minute)}
		if _, e = tx.Exec("INSERT INTO tokens(hash,actor_id,kind,session_id,body) VALUES(?,?,'resume',?,?)", digestSecret(cap), a.ID, session.ID, string(model.JSON(body))); e != nil {
			return "", 0, nil, e
		}
		if e = privatefs.WriteAtomic(filepath.Dir(s.store.Path), ref, model.JSON(map[string]string{"capability": cap})); e != nil {
			return "", 0, nil, e
		}
		result["handoffCapabilityRef"] = ref
	}
	return session.ID, session.Version, model.JSON(result), nil
}
func ownsResponsibility(tx rowReader, id, session, participant model.ID) error {
	if task, e := loadTask(tx, id); e == nil {
		if task.IntegratorSessionID != session || task.IntegratorID != participant {
			return model.Conflict("responsibility-changed")
		}
		return nil
	}
	assignment, e := loadAssignment(tx, id)
	if e != nil {
		return e
	}
	if assignment.ClaimSessionID != session || assignment.ClaimParticipantID != participant {
		return model.Conflict("responsibility-changed")
	}
	return nil
}
func (s *Service) resume(tx *store.Tx, a model.Actor, c model.Command, p resumePayload) (model.ID, int64, []byte, error) {
	target, e := loadSession(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	source, e := loadSession(tx, p.ExpectedSourceSessionID)
	if e != nil {
		return "", 0, nil, e
	}
	sourceProject, e := sessionProject(tx, source.ID)
	if e != nil {
		return "", 0, nil, e
	}
	targetProject, e := sessionProject(tx, target.ID)
	if e != nil {
		return "", 0, nil, e
	}
	if sourceProject != targetProject {
		return "", 0, nil, model.Forbidden()
	}
	if target.ParticipantID != source.ParticipantID || target.ID == source.ID || target.Status != "active" {
		return "", 0, nil, model.Conflict("invalid-resume-target")
	}
	if a.Kind != "human" && (target.ID != a.SessionID || a.ParticipantID != source.ParticipantID) {
		return "", 0, nil, model.Forbidden()
	}
	if e = version(p.SourceVersion, source.Version); e != nil {
		return "", 0, nil, e
	}
	if len(p.ResponsibilityIDs) == 0 || len(p.ResponsibilityIDs) > 2000 {
		return "", 0, nil, model.Invalid("explicit responsibilities required")
	}
	if p.Force && (a.Kind != "human" || p.Reason == "") {
		return "", 0, nil, model.Forbidden()
	}
	if source.Status != "ended" && !p.Force {
		if !strings.HasPrefix(p.HandoffCapabilityRef, "resume-caps/") || !strings.HasSuffix(p.HandoffCapabilityRef, ".json") {
			return "", 0, nil, model.Conflict("source-session-unknown")
		}
		raw, e := privatefs.ReadFile(filepath.Dir(s.store.Path), p.HandoffCapabilityRef)
		if e != nil {
			return "", 0, nil, model.Conflict("invalid-handoff-capability")
		}
		var file struct{ Capability string }
		if json.Unmarshal(raw, &file) != nil || file.Capability == "" {
			return "", 0, nil, model.Conflict("invalid-handoff-capability")
		}
		var body string
		var revoked int
		if e = tx.QueryRow("SELECT body,revoked FROM tokens WHERE hash=? AND kind='resume'", digestSecret(file.Capability)).Scan(&body, &revoked); e != nil {
			return "", 0, nil, model.Conflict("invalid-handoff-capability")
		}
		var cap resumeCapability
		if json.Unmarshal([]byte(body), &cap) != nil || revoked != 0 || !s.clock.Now().Before(cap.ExpiresAt) || cap.SourceSessionID != source.ID || cap.SourceVersion != source.Version || cap.TargetSessionID != target.ID || string(model.JSON(cap.ResponsibilityIDs)) != string(model.JSON(p.ResponsibilityIDs)) {
			return "", 0, nil, model.Conflict("invalid-handoff-capability")
		}
		if _, e = tx.Exec("UPDATE tokens SET revoked=1 WHERE hash=?", digestSecret(file.Capability)); e != nil {
			return "", 0, nil, e
		}
	}
	for i, id := range p.ResponsibilityIDs {
		if i > 0 && id == p.ResponsibilityIDs[i-1] {
			return "", 0, nil, model.Invalid("duplicate responsibility")
		}
		if e = ownsResponsibility(tx, id, source.ID, source.ParticipantID); e != nil {
			return "", 0, nil, e
		}
		if task, e := loadTask(tx, id); e == nil {
			if e = authorize(tx, a, task.ProjectID); e != nil {
				return "", 0, nil, e
			}
			task.IntegratorSessionID = target.ID
			task.IntegratorEpoch++
			if e = saveTask(tx, &task); e != nil {
				return "", 0, nil, e
			}
		} else {
			assignment, e := loadAssignment(tx, id)
			if e != nil {
				return "", 0, nil, e
			}
			task, e := loadTask(tx, assignment.TaskID)
			if e != nil {
				return "", 0, nil, e
			}
			if e = authorize(tx, a, task.ProjectID); e != nil {
				return "", 0, nil, e
			}
			assignment.ClaimSessionID = target.ID
			assignment.ClaimEpoch++
			if e = saveAssignment(tx, &assignment); e != nil {
				return "", 0, nil, e
			}
			if e = saveTask(tx, &task); e != nil {
				return "", 0, nil, e
			}
		}
	}
	source.Body["lastResumeTarget"] = target.ID
	if e = saveSession(tx, &source); e != nil {
		return "", 0, nil, e
	}
	return target.ID, target.Version, model.JSON(map[string]any{"id": target.ID, "sourceVersion": source.Version, "responsibilityIds": p.ResponsibilityIDs}), nil
}
