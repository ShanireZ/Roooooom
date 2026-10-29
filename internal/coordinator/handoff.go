package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

type disposition struct {
	AssignmentID    model.ID `json:"assignmentId"`
	Action          string   `json:"action"`
	TargetSessionID model.ID `json:"targetSessionId,omitempty"`
}
type handoffPayload struct {
	FromSessionID          model.ID      `json:"fromSessionId"`
	ToSessionID            model.ID      `json:"toSessionId"`
	HandoffID              model.ID      `json:"handoffId,omitempty"`
	AcceptedHandoffVersion int64         `json:"acceptedHandoffVersion,omitempty"`
	Dispositions           []disposition `json:"dispositions"`
	Reason                 string        `json:"reason"`
	Force                  bool          `json:"force,omitempty"`
}
type handoffOffer struct {
	ID                model.ID       `json:"handoffId"`
	Version           int64          `json:"version"`
	SourceTaskVersion int64          `json:"sourceTaskVersion"`
	Payload           handoffPayload `json:"payload"`
}

func clearProof(p handoffPayload) handoffPayload {
	p.HandoffID = ""
	p.AcceptedHandoffVersion = 0
	return p
}
func (s *Service) handoff(tx *store.Tx, a model.Actor, c model.Command, p handoffPayload) (model.ID, int64, []byte, error) {
	task, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = mutable(task); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, task.Version); e != nil {
		return "", 0, nil, e
	}
	if c.Meta.IntegratorEpoch < 1 {
		return "", 0, nil, model.Invalid("integratorEpoch required")
	}
	if c.Meta.IntegratorEpoch != task.IntegratorEpoch || p.FromSessionID != task.IntegratorSessionID {
		return "", 0, nil, model.Conflict("stale-integrator")
	}
	if p.Reason == "" || len(p.Reason) > 8192 || p.Dispositions == nil || len(p.Dispositions) > 2000 {
		return "", 0, nil, model.Invalid("reason and explicit dispositions required")
	}
	target, e := loadSession(tx, p.ToSessionID)
	if e != nil {
		return "", 0, nil, e
	}
	targetProject, e := sessionProject(tx, target.ID)
	if e != nil {
		return "", 0, nil, e
	}
	if targetProject != task.ProjectID {
		return "", 0, nil, model.Forbidden()
	}
	if target.ID == p.FromSessionID || target.ParticipantID == task.IntegratorID || target.Status != "active" || !participantInProject(tx, target.ParticipantID, task.ProjectID) {
		return "", 0, nil, model.Invalid("invalid integrator target")
	}
	if p.Force && a.Kind != "human" {
		return "", 0, nil, model.Forbidden()
	}
	if p.AcceptedHandoffVersion == 0 && !p.Force {
		if e = manager(a, task, c.Meta); e != nil {
			return "", 0, nil, e
		}
	} else if !p.Force && (a.Kind != "agent" || a.SessionID != target.ID || a.ParticipantID != target.ParticipantID) {
		return "", 0, nil, model.Forbidden()
	}
	claims, workers, e := validateHandoff(tx, task, target, p)
	if e != nil {
		return "", 0, nil, e
	}
	var offer handoffOffer
	if p.AcceptedHandoffVersion == 0 && !p.Force {
		var version int64
		if e = tx.QueryRow("SELECT COALESCE(MAX(version),0)+1 FROM handoffs WHERE task_id=?", task.ID).Scan(&version); e != nil {
			return "", 0, nil, e
		}
		offer = handoffOffer{s.ids.NewID(), version, task.Version, clearProof(p)}
		if _, e = tx.Exec("UPDATE handoffs SET state='superseded' WHERE task_id=? AND state='pending'", task.ID); e != nil {
			return "", 0, nil, e
		}
		if _, e = tx.Exec("INSERT INTO handoffs VALUES(?,?,?,'pending',?)", offer.ID, task.ID, offer.Version, string(model.JSON(offer))); e != nil {
			return "", 0, nil, e
		}
		return offer.ID, offer.Version, model.JSON(offer), nil
	}
	if !p.Force {
		var raw, state string
		if e = tx.QueryRow("SELECT body,state FROM handoffs WHERE id=? AND task_id=? AND version=?", p.HandoffID, task.ID, p.AcceptedHandoffVersion).Scan(&raw, &state); e != nil {
			return "", 0, nil, model.Conflict("handoff-changed")
		}
		if json.Unmarshal([]byte(raw), &offer) != nil || state != "pending" || offer.SourceTaskVersion != task.Version || string(model.JSON(offer.Payload)) != string(model.JSON(clearProof(p))) {
			return "", 0, nil, model.Conflict("handoff-changed")
		}
	}
	for _, d := range p.Dispositions {
		assignment := claims[d.AssignmentID]
		assignment.ClaimEpoch++
		switch d.Action {
		case "release":
			assignment.ClaimParticipantID = ""
			assignment.ClaimSessionID = ""
			assignment.State = "open"
		case "transfer", "continue-worker":
			target, e := loadSession(tx, d.TargetSessionID)
			if e != nil {
				return "", 0, nil, e
			}
			assignment.ClaimParticipantID = target.ParticipantID
			assignment.ClaimSessionID = target.ID
		}
		if e = saveAssignment(tx, &assignment); e != nil {
			return "", 0, nil, e
		}
	}
	if !workers[task.IntegratorID] {
		if e = retireAcceptedClaims(tx, task.ID, task.IntegratorID); e != nil {
			return "", 0, nil, e
		}
	}
	if _, e = tx.Exec("DELETE FROM memberships WHERE task_id=?", task.ID); e != nil {
		return "", 0, nil, e
	}
	if _, e = tx.Exec("INSERT INTO memberships VALUES(?,?,'integrator')", task.ID, target.ParticipantID); e != nil {
		return "", 0, nil, e
	}
	for participant := range workers {
		if _, e = tx.Exec("INSERT INTO memberships VALUES(?,?,'worker')", task.ID, participant); e != nil {
			return "", 0, nil, e
		}
	}
	task.IntegratorID = target.ParticipantID
	task.IntegratorSessionID = target.ID
	task.IntegratorEpoch++
	task.Phase = "active"
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	if p.Force {
		var version int64
		if e = tx.QueryRow("SELECT COALESCE(MAX(version),0)+1 FROM handoffs WHERE task_id=?", task.ID).Scan(&version); e != nil {
			return "", 0, nil, e
		}
		offer = handoffOffer{s.ids.NewID(), version, task.Version - 1, clearProof(p)}
		if _, e = tx.Exec("UPDATE handoffs SET state='superseded' WHERE task_id=? AND state='pending'", task.ID); e != nil {
			return "", 0, nil, e
		}
		if _, e = tx.Exec("INSERT INTO handoffs VALUES(?,?,?,'accepted',?)", offer.ID, task.ID, version, string(model.JSON(offer))); e != nil {
			return "", 0, nil, e
		}
	} else if _, e = tx.Exec("UPDATE handoffs SET state='accepted' WHERE id=?", offer.ID); e != nil {
		return "", 0, nil, e
	}
	return task.ID, task.Version, model.JSON(map[string]any{"task": task, "handoffId": offer.ID, "version": offer.Version, "forced": p.Force}), nil
}
func retireAcceptedClaims(tx *store.Tx, task, participant model.ID) error {
	rows, e := tx.Query("SELECT body FROM assignments WHERE task_id=? AND claim_participant_id=? AND state='accepted'", task, participant)
	if e != nil {
		return e
	}
	assignments := []model.Assignment{}
	for rows.Next() {
		var raw string
		var a model.Assignment
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return e
		}
		if e = json.Unmarshal([]byte(raw), &a); e != nil {
			rows.Close()
			return e
		}
		assignments = append(assignments, a)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, a := range assignments {
		a.ClaimParticipantID = ""
		a.ClaimSessionID = ""
		a.ClaimEpoch++
		if e = saveAssignment(tx, &a); e != nil {
			return e
		}
	}
	return nil
}
func validateHandoff(tx *store.Tx, task model.Task, target sessionRecord, p handoffPayload) (map[model.ID]model.Assignment, map[model.ID]bool, error) {
	rows, e := tx.Query("SELECT body FROM assignments WHERE task_id=? AND claim_participant_id=? AND state NOT IN('accepted','cancelled')", task.ID, task.IntegratorID)
	if e != nil {
		return nil, nil, e
	}
	claims := map[model.ID]model.Assignment{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return nil, nil, e
		}
		var assignment model.Assignment
		if e = json.Unmarshal([]byte(raw), &assignment); e != nil {
			rows.Close()
			return nil, nil, e
		}
		claims[assignment.ID] = assignment
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, nil, e
	}
	if len(p.Dispositions) != len(claims) {
		return nil, nil, model.Conflict("unhandled-claims")
	}
	workers := map[model.ID]bool{}
	rows, e = tx.Query("SELECT participant_id FROM memberships WHERE task_id=? AND role='worker' AND participant_id<>?", task.ID, target.ParticipantID)
	if e != nil {
		return nil, nil, e
	}
	for rows.Next() {
		var id model.ID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, nil, e
		}
		workers[id] = true
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, nil, e
	}
	seen := map[model.ID]bool{}
	for _, d := range p.Dispositions {
		if seen[d.AssignmentID] {
			return nil, nil, model.Invalid("duplicate disposition")
		}
		seen[d.AssignmentID] = true
		if _, ok := claims[d.AssignmentID]; !ok {
			return nil, nil, model.Conflict("claim-changed")
		}
		switch d.Action {
		case "release":
			if d.TargetSessionID != "" {
				return nil, nil, model.Invalid("release has no target")
			}
		case "transfer", "continue-worker":
			session, e := loadSession(tx, d.TargetSessionID)
			if e != nil {
				return nil, nil, e
			}
			targetProject, e := sessionProject(tx, session.ID)
			if e != nil {
				return nil, nil, e
			}
			if targetProject != task.ProjectID {
				return nil, nil, model.Forbidden()
			}
			if session.Status != "active" || !participantInProject(tx, session.ParticipantID, task.ProjectID) {
				return nil, nil, model.Invalid("invalid disposition target")
			}
			if d.Action == "continue-worker" && (session.ParticipantID != task.IntegratorID || session.ID != p.FromSessionID) {
				return nil, nil, model.Invalid("continue must retain the old integrator explicitly")
			}
			if session.ParticipantID != target.ParticipantID {
				workers[session.ParticipantID] = true
			}
		default:
			return nil, nil, model.Invalid("invalid disposition")
		}
	}
	if len(workers) > task.CollaboratorLimit {
		return nil, nil, model.Conflict("collaborator-capacity")
	}
	return claims, workers, nil
}
