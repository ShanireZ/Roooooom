package coordinator

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

func (s *Service) readMessages(tx readTx, a model.Actor, q model.Query) (model.View, error) {
	var view model.View
	if a.Kind == "agent" {
		for _, id := range a.ProjectIDs {
			if e := authorize(tx, a, id); e != nil {
				return view, e
			}
		}
	}
	if q.Kind == "message" {
		message, e := loadMessage(tx, q.TargetID)
		if e != nil {
			return view, e
		}
		if e = authorize(tx, a, message.ProjectID); e != nil {
			return view, e
		}
		if message.TaskID == "" && a.Kind != "human" && message.ActorID != a.ID {
			var n int
			if e = tx.QueryRow("SELECT count(*) FROM message_receipts WHERE message_id=? AND participant_id=?", message.ID, a.ParticipantID).Scan(&n); e != nil {
				return view, e
			}
			if n != 1 {
				return view, model.Forbidden()
			}
		}
		body := map[string]any{}
		if e = json.Unmarshal(model.JSON(message), &body); e != nil {
			return view, e
		}
		rows, e := tx.Query("SELECT participant_id,state,version,body FROM message_receipts WHERE message_id=? ORDER BY participant_id", message.ID)
		if e != nil {
			return view, e
		}
		receipts := []any{}
		for rows.Next() {
			var participant model.ID
			var state, raw string
			var version int64
			if e = rows.Scan(&participant, &state, &version, &raw); e != nil {
				rows.Close()
				return view, e
			}
			receipts = append(receipts, map[string]any{"participantId": participant, "state": state, "version": version, "metadata": json.RawMessage(raw)})
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return view, e
		}
		body["receipts"] = receipts
		view.Data = model.JSON(body)
		return view, nil
	}
	project := q.ProjectID
	taskID := q.TargetID
	if q.Kind == "thread" {
		if taskID != "" {
			task, e := loadTask(tx, taskID)
			if e != nil {
				return view, e
			}
			project = task.ProjectID
		} else if project == "" {
			return view, model.Invalid("project or task required")
		}
		if e := authorize(tx, a, project); e != nil {
			return view, e
		}
	}
	kind := q.Kind + "|" + string(project) + "|" + string(taskID)
	mark, e := s.decodeCursor(a, kind, q.Cursor)
	if e != nil {
		return view, e
	}
	rows, e := tx.Query(`SELECT e.seq,m.id,m.body,COALESCE(r.state,''),COALESCE(r.version,0) FROM messages m
 JOIN events e ON e.scope='message:'||m.id
 LEFT JOIN message_receipts r ON r.message_id=m.id AND r.participant_id=?
 WHERE e.seq>? AND (?='' OR m.project_id=?) AND (
  (?='inbox' AND r.participant_id=? AND r.state NOT IN('resolved','declined'))
  OR (?='thread' AND ((?<>'' AND m.task_id=?) OR (?='' AND m.task_id IS NULL AND (?='human' OR r.participant_id=? OR json_extract(m.body,'$.actorId')=?))))
 ) AND (?='human' OR m.project_id IN(SELECT value FROM json_each(?)))
 ORDER BY e.seq LIMIT ?`, a.ParticipantID, mark.Seq, project, project, q.Kind, a.ParticipantID, q.Kind, taskID, taskID, taskID, a.Kind, a.ParticipantID, a.ID, a.Kind, string(model.JSON(a.ProjectIDs)), q.Limit+1)
	if e != nil {
		return view, e
	}
	defer rows.Close()
	result := []any{}
	used := 0
	after := mark.Seq
	for rows.Next() {
		var seq, current int64
		var id model.ID
		var raw, state string
		if e = rows.Scan(&seq, &id, &raw, &state, &current); e != nil {
			return view, e
		}
		var body map[string]any
		if e = json.Unmarshal([]byte(raw), &body); e != nil {
			return view, e
		}
		body["receipt"] = map[string]any{"state": state, "version": current}
		encoded := model.JSON(body)
		if len(encoded) > 60*1024 {
			body = map[string]any{"id": id, "messageRef": id, "purpose": body["purpose"], "priority": body["priority"], "receipt": body["receipt"]}
			encoded = model.JSON(body)
		}
		if len(result) >= q.Limit || used+len(encoded) > 60*1024 {
			view.HasMore = true
			break
		}
		used += len(encoded)
		result = append(result, body)
		after = seq
	}
	if e = rows.Err(); e != nil {
		return view, e
	}
	view.Data = model.JSON(result)
	view.Cursor = s.encodeCursor(a, kind, after, "")
	return view, nil
}
func (s *Service) UnreadSummaries(ctx context.Context, a model.Actor) ([]any, error) {
	result := []any{}
	if a.Kind != "agent" {
		return result, nil
	}
	e := s.store.Read(ctx, func(reader store.Reader) error {
		tx := readTx{ctx, reader}
		for _, p := range a.ProjectIDs {
			if e := authorize(tx, a, p); e != nil {
				return e
			}
		}
		rows, e := tx.Query("SELECT m.id,json_extract(m.body,'$.purpose'),json_extract(m.body,'$.priority'),r.state,r.version FROM messages m JOIN message_receipts r ON r.message_id=m.id WHERE r.participant_id=? AND r.state IN('stored','fetched') AND m.project_id IN(SELECT value FROM json_each(?)) ORDER BY m.id LIMIT 5", a.ParticipantID, string(model.JSON(a.ProjectIDs)))
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id model.ID
			var purpose, priority, state string
			var version int64
			if e = rows.Scan(&id, &purpose, &priority, &state, &version); e != nil {
				return e
			}
			result = append(result, map[string]any{"messageRef": id, "purpose": purpose, "priority": priority, "state": state, "version": version})
		}
		return rows.Err()
	})
	return result, e
}
