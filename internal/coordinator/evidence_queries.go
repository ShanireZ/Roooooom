package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
)

func evidenceQuery(kind string) bool {
	switch kind {
	case "decisions", "decision", "deliveries", "delivery", "integrations", "candidate", "candidates", "acceptances", "completions", "offline-candidates", "offline-candidate":
		return true
	}
	return false
}
func (s *Service) readEvidence(tx readTx, a model.Actor, q model.Query) (model.View, error) {
	view := model.View{}
	switch q.Kind {
	case "offline-candidate":
		record, e := loadOfflineCandidate(tx, q.TargetID)
		if e != nil {
			return view, e
		}
		if e = authorize(tx, a, record.ProjectID); e != nil {
			return view, e
		}
		view.Data = model.JSON(record)
		return view, nil
	case "decision":
		request, e := loadDecisionRequest(tx, q.TargetID)
		if e != nil {
			return view, e
		}
		if e = authorize(tx, a, request.ProjectID); e != nil {
			return view, e
		}
		rows, e := tx.Query("SELECT body FROM decisions WHERE request_id=? ORDER BY version", request.ID)
		if e != nil {
			return view, e
		}
		history := []json.RawMessage{}
		for rows.Next() {
			var raw string
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return view, e
			}
			history = append(history, json.RawMessage(raw))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return view, e
		}
		view.Data = model.JSON(map[string]any{"request": withMessageKey(model.JSON(request)), "history": history})
		return view, nil
	case "delivery":
		delivery, e := loadDelivery(tx, q.TargetID)
		if e != nil {
			return view, e
		}
		task, e := loadTask(tx, delivery.TaskID)
		if e != nil {
			return view, e
		}
		if e = authorize(tx, a, task.ProjectID); e != nil {
			return view, e
		}
		assignment, e := loadAssignment(tx, delivery.AssignmentID)
		if e != nil {
			return view, e
		}
		stale := currentDelivery(tx, task, assignment, delivery) != nil
		view.Data = model.JSON(map[string]any{"delivery": delivery, "stale": stale, "evidenceSource": "agent-reported"})
		return view, nil
	case "candidate":
		candidate, e := loadCandidate(tx, q.TargetID)
		if e != nil {
			return view, e
		}
		task, e := loadTask(tx, candidate.TaskID)
		if e != nil {
			return view, e
		}
		if e = authorize(tx, a, task.ProjectID); e != nil {
			return view, e
		}
		_, current := candidateCurrent(tx, task, candidate.ID)
		view.Data = model.JSON(map[string]any{"candidate": candidate, "stale": current != nil, "evidenceSource": "agent-reported"})
		return view, nil
	}
	project := q.ProjectID
	if q.TargetID != "" {
		task, e := loadTask(tx, q.TargetID)
		if model.Status(e) == 404 && q.Kind == "decisions" {
			r, err := loadRequest(tx, q.TargetID)
			if err != nil {
				return view, err
			}
			project = r.ProjectID
			e = nil
		} else {
			project = task.ProjectID
		}
		if e != nil {
			return view, e
		}
	}
	if q.Kind != "decisions" && q.Kind != "offline-candidates" && q.TargetID == "" {
		return view, model.Invalid("task target required")
	}
	if project != "" {
		if e := authorize(tx, a, project); e != nil {
			return view, e
		}
	}
	kind := q.Kind + "|" + string(project) + "|" + string(q.TargetID)
	mark, e := s.decodeCursor(a, kind, q.Cursor)
	if e != nil {
		return view, e
	}
	query := ""
	args := []any{}
	switch q.Kind {
	case "offline-candidates":
		query = "SELECT o.id,o.body FROM offline_candidates o WHERE o.id>? AND (?='' OR o.project_id=?) AND (?='' OR EXISTS(SELECT 1 FROM json_each(o.body,'$.taskIds') WHERE value=?)) AND (?='human' OR o.project_id IN(SELECT value FROM json_each(?))) ORDER BY o.id LIMIT ?"
		args = []any{mark.AfterID, project, project, q.TargetID, q.TargetID, a.Kind, string(model.JSON(a.ProjectIDs)), q.Limit + 1}
	case "decisions":
		query = "SELECT r.id,r.body FROM decision_requests r WHERE r.id>? AND (?='' OR r.project_id=?) AND (?='' OR r.task_id=? OR r.request_id=? OR r.request_id IN(SELECT id FROM work_requests WHERE task_id=?)) AND (?='human' OR r.project_id IN(SELECT value FROM json_each(?))) ORDER BY r.id LIMIT ?"
		args = []any{mark.AfterID, project, project, q.TargetID, q.TargetID, q.TargetID, q.TargetID, a.Kind, string(model.JSON(a.ProjectIDs)), q.Limit + 1}
	case "deliveries":
		query = "SELECT d.id,d.body FROM deliveries d JOIN assignments a ON a.id=d.assignment_id WHERE a.task_id=? AND d.id>? ORDER BY d.id LIMIT ?"
		args = []any{q.TargetID, mark.AfterID, q.Limit + 1}
	case "integrations", "acceptances", "completions", "candidates":
		// Table names come exclusively from this internal allowlist.
		table := q.Kind
		if table == "candidates" {
			table = "acceptance_candidates"
		}
		query = "SELECT id,body FROM " + table + " WHERE task_id=? AND id>? ORDER BY id LIMIT ?"
		args = []any{q.TargetID, mark.AfterID, q.Limit + 1}
	default:
		return view, model.Invalid("unknown evidence query")
	}
	rows, e := tx.Query(query, args...)
	if e != nil {
		return view, e
	}
	defer rows.Close()
	result := []json.RawMessage{}
	after := mark.AfterID
	for rows.Next() {
		var id model.ID
		var raw string
		if e = rows.Scan(&id, &raw); e != nil {
			return view, e
		}
		if len(result) >= q.Limit {
			view.HasMore = true
			break
		}
		after = id
		if q.Kind == "decisions" {
			result = append(result, withMessageKey(json.RawMessage(raw)))
		} else {
			result = append(result, json.RawMessage(raw))
		}
	}
	if e = rows.Err(); e != nil {
		return view, e
	}
	view.Data = model.JSON(result)
	view.Cursor = s.encodeCursor(a, kind, 0, after)
	return view, nil
}
