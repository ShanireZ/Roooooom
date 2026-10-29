package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type offlineCandidate struct {
	ID            model.ID         `json:"id"`
	ProjectID     model.ID         `json:"projectId"`
	ActorID       model.ID         `json:"actorId"`
	SessionID     model.ID         `json:"sessionId"`
	Source        reconcilePayload `json:"source"`
	State         string           `json:"state"`
	Version       int64            `json:"version"`
	TaskIDs       []model.ID       `json:"taskIds"`
	ReceivedAt    time.Time        `json:"receivedAt"`
	Review        any              `json:"review,omitempty"`
	Authoritative bool             `json:"authoritative"`
}
type reconcileReviewPayload struct {
	Disposition  string   `json:"disposition"`
	Reason       string   `json:"reason"`
	EvidenceRefs []string `json:"evidenceRefs"`
}

func loadOfflineCandidate(tx rowReader, id model.ID) (offlineCandidate, error) {
	var record offlineCandidate
	var raw string
	if e := tx.QueryRow("SELECT body FROM offline_candidates WHERE id=?", id).Scan(&raw); e != nil {
		return record, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &record)
	return record, e
}
func relatedTasks(tx rowReader, c model.Command, p any) ([]model.ID, error) {
	result := []model.ID{}
	add := func(id model.ID) {
		if id != "" && !containsID(result, id) {
			if _, e := loadTask(tx, id); e == nil {
				result = append(result, id)
			}
		}
	}
	switch c.Kind {
	case "task.create", "project.create":
		return result, nil
	case "assignment.create":
		add(p.(*model.AssignmentCreate).TaskID)
	case "assignment.claim", "assignment.release", "assignment.update", "delivery.submit", "delivery.review":
		if assignment, e := loadAssignment(tx, c.TargetID); e == nil {
			add(assignment.TaskID)
		}
	case "decision.request":
		add(p.(*decisionRequestPayload).TaskID)
	case "decision.resolve", "decision.supersede", "decision.withdraw", "decision.followup":
		if request, e := loadDecisionRequest(tx, c.TargetID); e == nil {
			add(request.TaskID)
		}
	case "message.send":
		add(p.(*messagePayload).TaskID)
	case "receipt.update":
		if message, e := loadMessage(tx, c.TargetID); e == nil {
			add(message.TaskID)
		}
	case "session.resume":
		for _, id := range p.(*resumePayload).ResponsibilityIDs {
			if _, e := loadTask(tx, id); e == nil {
				add(id)
			} else if assignment, e := loadAssignment(tx, id); e == nil {
				add(assignment.TaskID)
			}
		}
	case "session.checkpoint", "session.end":
		rows, e := tx.Query("SELECT id FROM tasks WHERE integrator_session_id=? UNION SELECT task_id FROM assignments WHERE claim_session_id=?", c.TargetID, c.TargetID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id model.ID
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			result = append(result, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	default:
		add(c.TargetID)
	}
	return result, nil
}
func guardReconciliation(tx *store.Tx, a model.Actor, c model.Command, p any, project model.ID) error {
	switch c.Kind {
	case "task.create", "assignment.create", "assignment.claim", "assignment.release", "assignment.update", "collaborator.limit", "task.reopen", "task.cancel", "delivery.submit", "delivery.review", "integration.record", "acceptance.policy", "acceptance.candidate", "acceptance.accept", "task.complete":
	default:
		return nil
	}
	tasks, e := relatedTasks(tx, c, p)
	if e != nil {
		return e
	}
	var count int
	for _, task := range tasks {
		if e = tx.QueryRow("SELECT count(*) FROM offline_candidates o WHERE o.project_id=? AND json_extract(o.body,'$.state')='candidate' AND EXISTS(SELECT 1 FROM json_each(o.body,'$.taskIds') WHERE value=?)", project, task).Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			return model.Conflict("pending-reconciliation")
		}
	}
	if c.Kind == "task.create" {
		if e = tx.QueryRow("SELECT count(*) FROM offline_candidates WHERE project_id=? AND actor_id=? AND json_extract(body,'$.state')='candidate' AND json_extract(body,'$.source.originalCommand.kind')='task.create'", project, a.ID).Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			return model.Conflict("pending-reconciliation")
		}
	}
	return nil
}
func (s *Service) resolveOffline(tx *store.Tx, a model.Actor, c model.Command, p reconcileReviewPayload) (model.ID, int64, []byte, error) {
	record, e := loadOfflineCandidate(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, record.Version); e != nil {
		return "", 0, nil, e
	}
	if record.State != "candidate" {
		return "", 0, nil, model.Conflict("candidate-reviewed")
	}
	if p.Reason == "" || len(p.Reason) > 8192 || !validStrings(p.EvidenceRefs, true) {
		return "", 0, nil, model.Invalid("explicit reconciliation review required")
	}
	if p.Disposition != "reissue" && p.Disposition != "retain-artifact" && p.Disposition != "discard" {
		return "", 0, nil, model.Invalid("invalid reconciliation disposition")
	}
	if len(record.TaskIDs) == 0 {
		if a.Kind != "human" && a.ID != record.ActorID {
			return "", 0, nil, model.Forbidden()
		}
	} else {
		for _, id := range record.TaskIDs {
			task, e := loadTask(tx, id)
			if e != nil {
				return "", 0, nil, e
			}
			if e = manager(a, task, c.Meta); e != nil {
				return "", 0, nil, e
			}
		}
	}
	record.State = "reviewed"
	record.Version++
	record.Review = map[string]any{"disposition": p.Disposition, "reason": p.Reason, "evidenceRefs": p.EvidenceRefs, "actorId": a.ID, "actorKind": a.Kind, "sessionId": a.SessionID, "receivedAt": s.clock.Now(), "originalOperationApplied": false}
	if _, e = tx.Exec("UPDATE offline_candidates SET body=? WHERE id=?", string(model.JSON(record)), record.ID); e != nil {
		return "", 0, nil, e
	}
	return record.ID, record.Version, model.JSON(record), nil
}
