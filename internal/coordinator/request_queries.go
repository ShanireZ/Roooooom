package coordinator

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

func (s *Service) readRequests(ctx context.Context, a model.Actor, q model.Query) (view model.View, err error) {
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Limit < 1 || q.Limit > 200 {
		return view, model.Invalid("limit must be 1..200")
	}
	err = s.store.Read(ctx, func(reader store.Reader) error {
		tx := readTx{ctx, reader}
		if q.Kind == "request" {
			r, e := loadRequest(tx, q.TargetID)
			if e != nil {
				return e
			}
			if e = authorize(tx, a, r.ProjectID); e != nil {
				return e
			}
			view.Data = model.JSON(r)
			return nil
		}
		if q.ProjectID != "" {
			if e := authorize(tx, a, q.ProjectID); e != nil {
				return e
			}
		}
		if a.Kind == "agent" {
			for _, p := range a.ProjectIDs {
				if e := authorize(tx, a, p); e != nil {
					return e
				}
			}
		}
		kind := "requests|" + string(q.ProjectID)
		mark, e := s.decodeCursor(a, kind, q.Cursor)
		if e != nil {
			return e
		}
		rows, e := tx.Query("SELECT id,body FROM work_requests WHERE id>? AND (?='' OR project_id=?) AND (?='human' OR project_id IN(SELECT value FROM json_each(?))) ORDER BY id LIMIT ?", mark.AfterID, q.ProjectID, q.ProjectID, a.Kind, string(model.JSON(a.ProjectIDs)), q.Limit+1)
		if e != nil {
			return e
		}
		defer rows.Close()
		values := []json.RawMessage{}
		last := mark.AfterID
		bytes := 0
		for rows.Next() {
			var id model.ID
			var raw string
			if e = rows.Scan(&id, &raw); e != nil {
				return e
			}
			if len(values) >= q.Limit || len(values) > 0 && bytes+len(raw) > 1024*1024 {
				view.HasMore = true
				break
			}
			values = append(values, json.RawMessage(raw))
			bytes += len(raw)
			last = id
		}
		if e = rows.Err(); e != nil {
			return e
		}
		view.Data = model.JSON(values)
		if view.HasMore {
			view.Cursor = s.encodeCursor(a, kind, 0, last)
		}
		return nil
	})
	return view, err
}
