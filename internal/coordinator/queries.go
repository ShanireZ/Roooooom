package coordinator

import (
	"context"
	"database/sql"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/store"
)

type readTx struct {
	ctx    context.Context
	reader store.Reader
}

func (r readTx) QueryRow(q string, args ...any) *sql.Row {
	return r.reader.QueryRowContext(r.ctx, q, args...)
}
func (r readTx) Query(q string, args ...any) (*sql.Rows, error) {
	return r.reader.QueryContext(r.ctx, q, args...)
}
func (s *Service) Read(ctx context.Context, a model.Actor, q model.Query) (view model.View, err error) {
	if a.Kind != "agent" && a.Kind != "human" {
		return view, model.Forbidden()
	}
	if q.Kind == "requests" || q.Kind == "request" {
		return s.readRequests(ctx, a, q)
	}
	if q.Kind == "needs-you" || q.Kind == "activity" {
		return s.readRoom(ctx, a, q)
	}
	if q.Kind == "bindings" || q.Kind == "outbound-jobs" {
		if q.Limit == 0 {
			q.Limit = 100
		}
		if q.Limit < 1 || q.Limit > 200 {
			return view, model.Invalid("limit must be 1..200")
		}
		mark, e := s.decodeCursor(a, q.Kind+"|"+string(q.ProjectID), q.Cursor)
		if e != nil {
			return view, e
		}
		view, e = outbound.List(ctx, s.store, a, q.Kind, q.ProjectID, string(mark.AfterID), q.Limit)
		if e == nil && view.HasMore {
			view.Cursor = s.encodeCursor(a, q.Kind+"|"+string(q.ProjectID), 0, model.ID(view.Cursor))
		}
		return view, e
	}
	if q.Kind == "metrics" {
		if a.Kind != "human" {
			return view, model.Forbidden()
		}
		usage, e := s.store.Usage(ctx)
		if e != nil {
			return view, e
		}
		view.Data = model.JSON(map[string]any{"counters": s.metrics.Snapshot(), "storage": usage})
		return view, nil
	}
	if q.Limit < 0 || q.Limit > 200 {
		return view, model.Invalid("limit must be 1..200")
	}
	if q.Limit == 0 {
		q.Limit = 100
	}
	err = s.store.Read(ctx, func(reader store.Reader) error {
		tx := readTx{ctx, reader}
		if a.Kind == "agent" {
			if len(a.ProjectIDs) == 0 {
				return model.Forbidden()
			}
			for _, p := range a.ProjectIDs {
				if e := authorize(tx, a, p); e != nil {
					return e
				}
			}
		}
		if evidenceQuery(q.Kind) {
			var e error
			view, e = s.readEvidence(tx, a, q)
			return e
		}
		if q.Kind == "inbox" || q.Kind == "thread" || q.Kind == "message" {
			var e error
			view, e = s.readMessages(tx, a, q)
			return e
		}
		switch q.Kind {
		case "task":
			task, e := loadTask(tx, q.TargetID)
			if e != nil {
				return e
			}
			if e = authorize(tx, a, task.ProjectID); e != nil {
				return e
			}
			if e = s.taskView(tx, &task); e != nil {
				return e
			}
			times, e := s.taskTimes(tx, task.ProjectID)
			if e != nil {
				return e
			}
			applyTaskTimes(&task, times)
			view.Data = model.JSON(task)
			return nil
		case "assignment":
			assignment, e := loadAssignment(tx, q.TargetID)
			if e != nil {
				return e
			}
			task, e := loadTask(tx, assignment.TaskID)
			if e != nil {
				return e
			}
			if e = authorize(tx, a, task.ProjectID); e != nil {
				return e
			}
			ready, e := assignmentCanStart(tx, task, assignment)
			if e != nil {
				return e
			}
			assignment.CanStart = ready
			view.Data = model.JSON(assignment)
			return nil
		}
		project := q.ProjectID
		if q.Kind == "assignments" && q.TargetID != "" {
			task, e := loadTask(tx, q.TargetID)
			if e != nil {
				return e
			}
			project = task.ProjectID
		}
		if q.Kind != "projects" && project == "" && q.Kind != "tasks" && q.Kind != "attention" && q.Kind != "sessions" && !(q.Kind == "participants" && a.Kind == "human") {
			return model.Invalid("unknown query")
		}
		if project != "" {
			if e := authorize(tx, a, project); e != nil {
				return e
			}
		}
		cursorKind := q.Kind + "|" + string(project) + "|" + string(q.TargetID)
		mark, cursorErr := s.decodeCursor(a, cursorKind, q.Cursor)
		if cursorErr != nil {
			return cursorErr
		}
		var rows *sql.Rows
		var e error
		switch q.Kind {
		case "projects":
			rows, e = tx.Query("SELECT p.id,p.body FROM projects p WHERE p.id>? AND (?='human' OR (EXISTS(SELECT 1 FROM participant_projects m WHERE m.project_id=p.id AND m.participant_id=?) AND p.id IN(SELECT value FROM json_each(?)))) ORDER BY p.id LIMIT ?", mark.AfterID, a.Kind, a.ParticipantID, string(model.JSON(a.ProjectIDs)), q.Limit+1)
		case "tasks", "attention":
			rows, e = tx.Query("SELECT t.id,t.body FROM tasks t WHERE t.id>? AND (?='' OR t.project_id=?) AND (?='human' OR (EXISTS(SELECT 1 FROM participant_projects m WHERE m.project_id=t.project_id AND m.participant_id=?) AND t.project_id IN(SELECT value FROM json_each(?)))) ORDER BY t.id LIMIT ?", mark.AfterID, project, project, a.Kind, a.ParticipantID, string(model.JSON(a.ProjectIDs)), q.Limit+1)
		case "assignments":
			if q.TargetID != "" {
				rows, e = tx.Query("SELECT id,body FROM assignments WHERE task_id=? AND id>? ORDER BY id LIMIT ?", q.TargetID, string(mark.AfterID), q.Limit+1)
			} else {
				// A room reads every open task's assignments in one request.
				rows, e = tx.Query("SELECT a.id,a.body FROM assignments a JOIN tasks t ON t.id=a.task_id WHERE t.project_id=? AND t.phase IN('ready','active','integrating','review') AND a.id>? ORDER BY a.id LIMIT ?", project, string(mark.AfterID), q.Limit+1)
			}
		case "sessions":
			rows, e = tx.Query("SELECT s.id,s.body FROM sessions s WHERE s.id>? AND EXISTS(SELECT 1 FROM participant_projects m WHERE m.participant_id=s.participant_id AND (?='' OR m.project_id=?) AND (?='human' OR (EXISTS(SELECT 1 FROM participant_projects own WHERE own.project_id=m.project_id AND own.participant_id=?) AND m.project_id IN(SELECT value FROM json_each(?))))) ORDER BY s.id LIMIT ?", mark.AfterID, project, project, a.Kind, a.ParticipantID, string(model.JSON(a.ProjectIDs)), q.Limit+1)
		case "participants":
			rows, e = tx.Query("SELECT DISTINCT p.id,p.body FROM participants p JOIN participant_projects m ON m.participant_id=p.id WHERE (?='' OR m.project_id=?) AND p.id>? ORDER BY p.id LIMIT ?", project, project, string(mark.AfterID), q.Limit+1)
		default:
			return model.Invalid("unknown query")
		}
		if e != nil {
			return e
		}
		// Finish rows before enrichment; the transaction keeps a consistent snapshot.
		type record struct {
			id   model.ID
			body json.RawMessage
		}
		records := []record{}
		rawBytes := 0
		for rows.Next() {
			var id model.ID
			var raw string
			if e = rows.Scan(&id, &raw); e != nil {
				rows.Close()
				return e
			}
			if len(records) > 0 && rawBytes+len(raw) > 1024*1024 {
				view.HasMore = true
				break
			}
			records = append(records, record{id, json.RawMessage(raw)})
			rawBytes += len(raw)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(records) > q.Limit {
			view.HasMore = true
			records = records[:q.Limit]
		}
		result := []any{}
		timesByProject := map[model.ID]map[model.ID]activityTimes{}
		after := mark.AfterID
		encodedBytes := 0
		for _, r := range records {
			previous := len(result)
			switch q.Kind {
			case "projects":
				var body map[string]any
				if e = json.Unmarshal(r.body, &body); e != nil {
					return e
				}
				rows, e := tx.Query("SELECT id,version,body FROM repositories WHERE project_id=? ORDER BY id", r.id)
				if e != nil {
					return e
				}
				repositories := []any{}
				for rows.Next() {
					var repoID model.ID
					var version int64
					var raw string
					if e = rows.Scan(&repoID, &version, &raw); e != nil {
						rows.Close()
						return e
					}
					var repo map[string]any
					if e = json.Unmarshal([]byte(raw), &repo); e != nil {
						rows.Close()
						return e
					}
					repo["id"] = repoID
					repo["projectId"] = r.id
					repo["version"] = version
					repositories = append(repositories, repo)
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					return e
				}
				body["id"] = r.id
				body["version"] = 1
				body["repositories"] = repositories
				if a.Kind == "human" {
					var grants int
					if e = tx.QueryRow("SELECT count(*) FROM tokens WHERE kind='enrollment' AND project_id=? AND revoked=0", r.id).Scan(&grants); e != nil {
						return e
					}
					body["enrollmentEnabled"] = grants > 0
				}
				created, e := projectCreatedAt(tx, r.id)
				if e != nil {
					return e
				}
				if !created.IsZero() {
					body["createdAt"] = created
				}
				result = append(result, body)
			case "tasks", "attention":
				var task model.Task
				if e = json.Unmarshal(r.body, &task); e != nil {
					return e
				}
				if e = s.taskView(tx, &task); e != nil {
					return e
				}
				times, ok := timesByProject[task.ProjectID]
				if !ok {
					if times, e = s.taskTimes(tx, task.ProjectID); e != nil {
						return e
					}
					timesByProject[task.ProjectID] = times
				}
				applyTaskTimes(&task, times)
				if q.Kind == "tasks" || len(task.Attention) > 0 {
					result = append(result, task)
				}
			case "assignments":
				var assignment model.Assignment
				if e = json.Unmarshal(r.body, &assignment); e != nil {
					return e
				}
				task, loadErr := loadTask(tx, assignment.TaskID)
				if loadErr != nil {
					return loadErr
				}
				assignment.CanStart, e = assignmentCanStart(tx, task, assignment)
				if e != nil {
					return e
				}
				result = append(result, assignment)
			case "sessions":
				body, e := s.sessionView(tx, r.id)
				if e != nil {
					return e
				}
				result = append(result, body)
			default:
				var body map[string]any
				if e = json.Unmarshal(r.body, &body); e != nil {
					return e
				}
				body["id"] = r.id
				result = append(result, body)
			}
			if len(result) > previous {
				size := len(model.JSON(result[len(result)-1]))
				if previous > 0 && encodedBytes+size > 1024*1024 {
					result = result[:previous]
					view.HasMore = true
					break
				}
				encodedBytes += size
			}
			after = r.id
		}
		view.Data = model.JSON(result)
		view.Cursor = s.encodeCursor(a, cursorKind, 0, after)
		return nil
	})
	return view, err
}
func assignmentCanStart(tx rowReader, task model.Task, assignment model.Assignment) (bool, error) {
	ready, e := dependenciesReady(tx, assignment)
	if e != nil {
		return false, e
	}
	if !ready || task.IntegratorID == "" || task.Phase == "done" || task.Phase == "cancelled" || assignment.State == "accepted" || assignment.State == "cancelled" {
		return false, nil
	}
	var members int
	if e = tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", task.IntegratorID, task.ProjectID).Scan(&members); e != nil {
		return false, e
	}
	return members == 1 && guardDecisions(tx, task, assignment.ID, "implementation") == nil, nil
}
func (s *Service) taskView(tx readTx, t *model.Task) error {
	t.RecordRefs = []model.RecordRef{}
	refs, e := tx.Query("SELECT j.binding_id,json_extract(r.body,'$.remoteId'),json_extract(r.body,'$.remoteUrl'),coalesce(json_extract(j.body,'$.taskVersion'),0) FROM external_jobs j JOIN external_receipts r ON r.job_id=j.id WHERE j.task_id=? AND j.kind='github.issue' ORDER BY j.rowid", t.ID)
	if e != nil {
		return e
	}
	for refs.Next() {
		var ref model.RecordRef
		if e = refs.Scan(&ref.BindingID, &ref.Number, &ref.URL, &ref.TaskVersion); e != nil {
			refs.Close()
			return e
		}
		t.RecordRefs = append(t.RecordRefs, ref)
	}
	e = refs.Err()
	refs.Close()
	if e != nil {
		return e
	}
	t.ParticipantIDs = []model.ID{}
	members, e := tx.Query("SELECT participant_id FROM memberships WHERE task_id=? ORDER BY participant_id", t.ID)
	if e != nil {
		return e
	}
	for members.Next() {
		var id model.ID
		if e = members.Scan(&id); e != nil {
			members.Close()
			return e
		}
		t.ParticipantIDs = append(t.ParticipantIDs, id)
	}
	e = members.Err()
	members.Close()
	if e != nil {
		return e
	}
	if e := tx.QueryRow("SELECT count(*) FROM memberships WHERE task_id=? AND role='worker'", t.ID).Scan(&t.CollaboratorCount); e != nil {
		return e
	}
	t.Attention = []string{}
	rows, e := tx.Query("SELECT id FROM offline_candidates WHERE json_extract(body,'$.state')='candidate' AND EXISTS(SELECT 1 FROM json_each(body,'$.taskIds') WHERE value=?)", t.ID)
	if e != nil {
		return e
	}
	for rows.Next() {
		var id model.ID
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		t.Attention = append(t.Attention, "pending-reconciliation:"+string(id))
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	decisions, e := currentDecisions(tx, t.ID, "")
	if e != nil {
		return e
	}
	t.DecisionVersions = decisions
	requests, e := decisionRequests(tx, t.ID)
	if e != nil {
		return e
	}
	for _, request := range requests {
		if request.State == "pending" || request.FollowUpRequired && !request.FollowUpComplete || request.ResolutionOutcome == "rejected" || request.ResolutionOutcome == "revise" {
			t.Attention = append(t.Attention, "decision-attention:"+string(request.ID))
		}
	}
	if t.IntegratorID == "" {
		t.Attention = append(t.Attention, "integrator-needed")
	}
	enrolled := false
	if t.IntegratorID != "" {
		var n int
		if e := tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", t.IntegratorID, t.ProjectID).Scan(&n); e != nil {
			return e
		}
		enrolled = n == 1
		if !enrolled {
			t.Attention = append(t.Attention, "integrator-revoked")
		}
	}
	if t.IntegratorSessionID != "" {
		session, e := s.sessionView(tx, t.IntegratorSessionID)
		if e != nil {
			return e
		}
		if session["presence"] == "unknown" {
			t.Attention = append(t.Attention, "integrator-presence-stale")
		}
		if session["presence"] == "ended" {
			t.Attention = append(t.Attention, "integrator-session-ended")
		}
	}
	riskBlocked := false
	if t.Risk == "high" {
		if guardDecisions(tx, *t, "", "implementation") != nil {
			riskBlocked = true
			t.Attention = append(t.Attention, "owner-decision-needed")
		}
	}
	t.CanStart = !riskBlocked && enrolled && t.Phase != "done" && t.Phase != "cancelled" && guardDecisions(tx, *t, "", "implementation") == nil
	return nil
}
