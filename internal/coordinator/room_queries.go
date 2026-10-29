package coordinator

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

// needsYouItem is one thing only the owner can move forward.
type needsYouItem struct {
	Kind              string    `json:"kind"`
	ProjectID         model.ID  `json:"projectId"`
	TaskID            model.ID  `json:"taskId,omitempty"`
	RequestID         model.ID  `json:"requestId,omitempty"`
	DecisionRequestID model.ID  `json:"decisionRequestId,omitempty"`
	CandidateID       model.ID  `json:"candidateId,omitempty"`
	Code              string    `json:"code,omitempty"`
	Summary           string    `json:"summary"`
	Since             time.Time `json:"since"`
}

// activityEntry is one event in a room's feed; message bodies come from the thread query.
type activityEntry struct {
	Seq        int64     `json:"seq"`
	Kind       string    `json:"kind"`
	ResourceID model.ID  `json:"resourceId"`
	TaskID     model.ID  `json:"taskId,omitempty"`
	RequestID  model.ID  `json:"requestId,omitempty"`
	ActorID    model.ID  `json:"actorId,omitempty"`
	ActorKind  string    `json:"actorKind,omitempty"`
	ReceivedAt time.Time `json:"receivedAt"`
	Imported   bool      `json:"imported"`
}

// readRoom serves the owner-only room reads. They aggregate existing records
// and grant nothing.
func (s *Service) readRoom(ctx context.Context, a model.Actor, q model.Query) (view model.View, err error) {
	if a.Kind != "human" {
		return view, model.Forbidden()
	}
	err = s.store.Read(ctx, func(reader store.Reader) error {
		tx := readTx{ctx, reader}
		if q.Kind == "needs-you" {
			items, e := s.needsYou(tx, q.ProjectID)
			view.Data = model.JSON(items)
			return e
		}
		return s.activity(tx, a, q, &view)
	})
	return view, err
}

func (s *Service) needsYou(tx readTx, project model.ID) ([]needsYouItem, error) {
	items := []needsYouItem{}
	ids, e := idList(tx, "SELECT id FROM decision_requests WHERE state='pending' AND (?='' OR project_id=?) ORDER BY id", project, project)
	if e != nil {
		return nil, e
	}
	for _, id := range ids {
		r, e := loadDecisionRequest(tx, id)
		if e != nil {
			return nil, e
		}
		items = append(items, needsYouItem{Kind: "decision", ProjectID: r.ProjectID, TaskID: r.TaskID, RequestID: r.RequestID, DecisionRequestID: r.ID, Summary: r.Question, Since: r.ReceivedAt})
	}
	ids, e = idList(tx, "SELECT id FROM tasks WHERE phase NOT IN('done','cancelled') AND (?='' OR project_id=?) ORDER BY id", project, project)
	if e != nil {
		return nil, e
	}
	timesByProject := map[model.ID]map[model.ID]activityTimes{}
	for _, id := range ids {
		task, e := loadTask(tx, id)
		if e != nil {
			return nil, e
		}
		if e = s.taskView(tx, &task); e != nil {
			return nil, e
		}
		times, ok := timesByProject[task.ProjectID]
		if !ok {
			if times, e = taskTimes(tx, task.ProjectID); e != nil {
				return nil, e
			}
			timesByProject[task.ProjectID] = times
		}
		updated := times[task.ID].Updated
		if task.AcceptancePolicy.Mode == "human" && task.CurrentCandidateID != "" {
			ready, since, e := humanAcceptanceReady(tx, task)
			if e != nil {
				return nil, e
			}
			if ready {
				items = append(items, needsYouItem{Kind: "acceptance", ProjectID: task.ProjectID, TaskID: task.ID, CandidateID: task.CurrentCandidateID, Summary: task.Goal, Since: since})
			}
		}
		for _, code := range task.Attention {
			if code != "integrator-revoked" && code != "integrator-session-ended" && !strings.HasPrefix(code, "pending-reconciliation:") {
				continue
			}
			since := updated
			if code == "integrator-session-ended" {
				if seen := s.lastSeen(tx, task.IntegratorSessionID); !seen.IsZero() {
					since = seen
				}
			}
			items = append(items, needsYouItem{Kind: "blocker", ProjectID: task.ProjectID, TaskID: task.ID, Code: code, Summary: task.Goal, Since: since})
		}
	}
	blocked, e := s.blockedRequests(tx, project)
	if e != nil {
		return nil, e
	}
	items = append(items, blocked...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Since.Before(items[j].Since) })
	return items, nil
}

// humanAcceptanceReady applies the same staleness and prior-acceptance rules
// as the candidate query and notifications.
func humanAcceptanceReady(tx readTx, task model.Task) (bool, time.Time, error) {
	if _, stale := candidateCurrent(tx, task, task.CurrentCandidateID); stale != nil {
		return false, time.Time{}, nil
	}
	var accepted int
	if e := tx.QueryRow("SELECT count(*) FROM acceptances WHERE candidate_id=? AND json_extract(body,'$.actorKind')='human'", task.CurrentCandidateID).Scan(&accepted); e != nil {
		return false, time.Time{}, e
	}
	if accepted > 0 {
		return false, time.Time{}, nil
	}
	var at string
	e := tx.QueryRow("SELECT "+eventTimeSQL+" FROM events WHERE scope=? AND json_extract(body,'$.resourceId')=? ORDER BY seq LIMIT 1", task.ProjectID, task.CurrentCandidateID).Scan(&at)
	if e != nil && e != sql.ErrNoRows {
		return false, time.Time{}, e
	}
	return true, parseEventTime(at), nil
}

// blockedRequests lists open intakes whose preparer ended or lost access.
func (s *Service) blockedRequests(tx readTx, project model.ID) ([]needsYouItem, error) {
	items := []needsYouItem{}
	rows, e := tx.Query("SELECT body FROM work_requests WHERE state='preparing' AND (?='' OR project_id=?) ORDER BY id", project, project)
	if e != nil {
		return nil, e
	}
	requests := []model.WorkRequest{}
	for rows.Next() {
		var raw string
		var r model.WorkRequest
		if e = rows.Scan(&raw); e == nil {
			e = json.Unmarshal([]byte(raw), &r)
		}
		if e != nil {
			rows.Close()
			return nil, e
		}
		requests = append(requests, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for _, r := range requests {
		code := ""
		var members int
		if e = tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", r.PreparerParticipantID, r.ProjectID).Scan(&members); e != nil {
			return nil, e
		}
		since := r.CreatedAt
		if members == 0 {
			code = "preparer-revoked"
		} else if r.PreparerSessionID != "" {
			session, e := loadSession(tx, r.PreparerSessionID)
			if e != nil {
				return nil, e
			}
			if session.Status == "ended" {
				code = "preparer-session-ended"
				if seen := s.lastSeen(tx, r.PreparerSessionID); !seen.IsZero() {
					since = seen
				}
			}
		}
		if code != "" {
			items = append(items, needsYouItem{Kind: "blocker", ProjectID: r.ProjectID, RequestID: r.ID, Code: code, Summary: r.Text, Since: since})
		}
	}
	return items, nil
}

func (s *Service) lastSeen(tx readTx, session model.ID) time.Time {
	if session == "" {
		return time.Time{}
	}
	view, e := s.sessionView(tx, session)
	if e != nil {
		return time.Time{}
	}
	seen, _ := view["lastSeenAt"].(time.Time)
	return seen
}

func (s *Service) activity(tx readTx, a model.Actor, q model.Query, view *model.View) error {
	if q.ProjectID == "" {
		return model.Invalid("projectId required")
	}
	if e := authorize(tx, a, q.ProjectID); e != nil {
		return e
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	if q.Limit < 1 || q.Limit > 200 {
		return model.Invalid("limit must be 1..200")
	}
	cursorKind := "activity|" + string(q.ProjectID)
	mark, e := s.decodeCursor(a, cursorKind, q.Cursor)
	if e != nil {
		return e
	}
	tasks, e := resourceTasks(tx, q.ProjectID)
	if e != nil {
		return e
	}
	requests, e := pairs(tx, q.ProjectID, "SELECT id,id FROM work_requests WHERE project_id=?", "SELECT id,request_id FROM decision_requests WHERE project_id=? AND request_id IS NOT NULL")
	if e != nil {
		return e
	}
	rows, e := tx.Query("SELECT seq,body,"+eventTimeSQL+" FROM events WHERE (scope=? OR scope IN(SELECT 'message:'||id FROM messages WHERE project_id=?)) AND (?=0 OR seq<?) ORDER BY seq DESC LIMIT ?", q.ProjectID, q.ProjectID, mark.Seq, mark.Seq, q.Limit+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	entries := []activityEntry{}
	for rows.Next() {
		var seq int64
		var raw, at string
		if e = rows.Scan(&seq, &raw, &at); e != nil {
			return e
		}
		if len(entries) == q.Limit {
			view.HasMore = true
			break
		}
		var body struct {
			Kind       string   `json:"kind"`
			ResourceID model.ID `json:"resourceId"`
			ActorID    model.ID `json:"actorId"`
			ActorKind  string   `json:"actorKind"`
			Imported   bool     `json:"imported"`
		}
		if e = json.Unmarshal([]byte(raw), &body); e != nil {
			return e
		}
		entries = append(entries, activityEntry{Seq: seq, Kind: body.Kind, ResourceID: body.ResourceID, TaskID: tasks[body.ResourceID], RequestID: requests[body.ResourceID], ActorID: body.ActorID, ActorKind: body.ActorKind, ReceivedAt: parseEventTime(at), Imported: body.Imported})
	}
	if e = rows.Err(); e != nil {
		return e
	}
	view.Data = model.JSON(entries)
	if view.HasMore {
		view.Cursor = s.encodeCursor(a, cursorKind, entries[len(entries)-1].Seq, "")
	}
	return nil
}

func idList(tx readTx, query string, args ...any) ([]model.ID, error) {
	rows, e := tx.Query(query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	ids := []model.ID{}
	for rows.Next() {
		var id model.ID
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func pairs(tx readTx, project model.ID, queries ...string) (map[model.ID]model.ID, error) {
	result := map[model.ID]model.ID{}
	for _, query := range queries {
		rows, e := tx.Query(query, project)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var key, value model.ID
			if e = rows.Scan(&key, &value); e != nil {
				rows.Close()
				return nil, e
			}
			result[key] = value
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}
