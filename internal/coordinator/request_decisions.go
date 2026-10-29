package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

// Original question subjects remain unchanged; the conversion link resolves
// their effective task without creating a placeholder or a second decision.
func decisionSubject(tx rowReader, taskID, requestID model.ID) (model.Task, model.WorkRequest, error) {
	var task model.Task
	var request model.WorkRequest
	var e error
	if (taskID == "") == (requestID == "") {
		return task, request, model.Invalid("exactly one task or request subject required")
	}
	if taskID != "" {
		task, e = loadTask(tx, taskID)
		return task, request, e
	}
	request, e = loadRequest(tx, requestID)
	if e != nil {
		return task, request, e
	}
	if request.TaskID != "" {
		task, e = loadTask(tx, request.TaskID)
		if e == nil && task.ProjectID != request.ProjectID {
			e = model.Forbidden()
		}
	}
	return task, request, e
}
func requestDecisionVersions(tx rowReader, r model.WorkRequest, m model.Mutation) (map[model.ID]int64, error) {
	rows, e := tx.Query("SELECT body FROM decision_requests WHERE request_id=? ORDER BY id", r.ID)
	if e != nil {
		return nil, e
	}
	questions := []decisionRequestRecord{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return nil, e
		}
		var q decisionRequestRecord
		if e = json.Unmarshal([]byte(raw), &q); e != nil {
			rows.Close()
			return nil, e
		}
		questions = append(questions, q)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	versions := map[model.ID]int64{}
	for _, q := range questions {
		if q.State == "pending" || q.FollowUpRequired && !q.FollowUpComplete || q.ResolutionOutcome == "rejected" || q.ResolutionOutcome == "revise" {
			return nil, model.Conflict("decision-pending")
		}
		if q.State == "resolved" {
			if m.DecisionVersions[q.ID] != q.DecisionVersion {
				return nil, model.Conflict("stale-decision")
			}
			versions[q.ID] = q.DecisionVersion
		}
	}
	for id, v := range m.DecisionVersions {
		if versions[id] != v {
			return nil, model.Conflict("stale-decision")
		}
	}
	return versions, nil
}
func (s *Service) cancelRequestQuestions(tx *store.Tx, a model.Actor, r model.WorkRequest) error {
	rows, e := tx.Query("SELECT id FROM decision_requests WHERE request_id=? AND state='pending' ORDER BY id", r.ID)
	if e != nil {
		return e
	}
	ids := []model.ID{}
	for rows.Next() {
		var id model.ID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		question, e := loadDecisionRequest(tx, id)
		if e != nil {
			return e
		}
		question.State = "withdrawn"
		if e = saveDecisionRequest(tx, &question); e != nil {
			return e
		}
		if _, e = tx.AppendEvent(string(r.ProjectID), model.JSON(map[string]any{"kind": "decision.withdraw", "resourceId": question.ID, "version": question.Version, "actorId": a.ID, "actorKind": a.Kind, "receivedAt": s.clock.Now(), "reason": "source request cancelled", "sourceRequestId": r.ID})); e != nil {
			return e
		}
	}
	return nil
}
