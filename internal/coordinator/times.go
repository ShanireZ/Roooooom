package coordinator

import (
	"database/sql"
	"time"

	"roooooom.local/roooooom/internal/model"
)

// activityTimes are view-only task times derived from the never-pruned events table.
type activityTimes struct {
	Created, Updated time.Time
	Completed        *time.Time
}

// eventTimeSQL prefers the service clock recorded in the event body.
const eventTimeSQL = "coalesce(json_extract(body,'$.receivedAt'),received_at)"

func parseEventTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, raw)
	return t
}

// resourceTasks maps every task-scoped record of a project to its task.
func resourceTasks(tx readTx, projectID model.ID) (map[model.ID]model.ID, error) {
	owners := map[model.ID]model.ID{}
	for _, q := range []string{
		"SELECT id,id FROM tasks WHERE project_id=?",
		"SELECT a.id,a.task_id FROM assignments a JOIN tasks t ON t.id=a.task_id WHERE t.project_id=?",
		"SELECT d.id,a.task_id FROM deliveries d JOIN assignments a ON a.id=d.assignment_id JOIN tasks t ON t.id=a.task_id WHERE t.project_id=?",
		"SELECT x.id,x.task_id FROM integrations x JOIN tasks t ON t.id=x.task_id WHERE t.project_id=?",
		"SELECT x.id,x.task_id FROM acceptance_candidates x JOIN tasks t ON t.id=x.task_id WHERE t.project_id=?",
		"SELECT x.id,x.task_id FROM acceptances x JOIN tasks t ON t.id=x.task_id WHERE t.project_id=?",
		"SELECT x.id,x.task_id FROM completions x JOIN tasks t ON t.id=x.task_id WHERE t.project_id=?",
		"SELECT x.id,x.task_id FROM handoffs x JOIN tasks t ON t.id=x.task_id WHERE t.project_id=?",
		"SELECT id,task_id FROM decision_requests WHERE project_id=? AND task_id IS NOT NULL",
		"SELECT id,task_id FROM messages WHERE project_id=? AND task_id IS NOT NULL",
	} {
		rows, e := tx.Query(q, projectID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id, task model.ID
			if e = rows.Scan(&id, &task); e != nil {
				rows.Close()
				return nil, e
			}
			owners[id] = task
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return owners, nil
}

// resourceTaskSQL finds the task that owns one record, by primary key.
const resourceTaskSQL = `SELECT id FROM tasks WHERE id=?1
UNION ALL SELECT task_id FROM assignments WHERE id=?1
UNION ALL SELECT a.task_id FROM deliveries d JOIN assignments a ON a.id=d.assignment_id WHERE d.id=?1
UNION ALL SELECT task_id FROM integrations WHERE id=?1
UNION ALL SELECT task_id FROM acceptance_candidates WHERE id=?1
UNION ALL SELECT task_id FROM acceptances WHERE id=?1
UNION ALL SELECT task_id FROM completions WHERE id=?1
UNION ALL SELECT task_id FROM handoffs WHERE id=?1
UNION ALL SELECT task_id FROM decision_requests WHERE id=?1 AND task_id IS NOT NULL
UNION ALL SELECT task_id FROM messages WHERE id=?1 AND task_id IS NOT NULL
LIMIT 1`

// foldTaskTimes attributes the project's events after seq to the tasks that own
// their resources (events_scope_seq). From the start it maps every record at once;
// a later fold looks up only the records its few new events name.
func foldTaskTimes(tx readTx, projectID model.ID, after int64, times map[model.ID]activityTimes) error {
	type event struct{ at, resource, kind string }
	rows, e := tx.Query("SELECT "+eventTimeSQL+",json_extract(body,'$.resourceId'),json_extract(body,'$.kind') FROM events WHERE scope=? AND seq>? ORDER BY seq", projectID, after)
	if e != nil {
		return e
	}
	events := []event{}
	for rows.Next() {
		var at string
		var resource, kind sql.NullString
		if e = rows.Scan(&at, &resource, &kind); e != nil {
			rows.Close()
			return e
		}
		events = append(events, event{at, resource.String, kind.String})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	owners := map[model.ID]model.ID{}
	if after == 0 {
		if owners, e = resourceTasks(tx, projectID); e != nil {
			return e
		}
	} else {
		for _, ev := range events {
			id := model.ID(ev.resource)
			if _, seen := owners[id]; seen || id == "" {
				continue
			}
			var task model.ID
			if e = tx.QueryRow(resourceTaskSQL, id).Scan(&task); e != nil && e != sql.ErrNoRows {
				return e
			}
			owners[id] = task
		}
	}
	for _, ev := range events {
		task := owners[model.ID(ev.resource)]
		if task == "" {
			continue
		}
		moment := parseEventTime(ev.at)
		t := times[task]
		if t.Created.IsZero() || moment.Before(t.Created) {
			t.Created = moment
		}
		if moment.After(t.Updated) {
			t.Updated = moment
		}
		if ev.kind == "task.complete" && model.ID(ev.resource) == task {
			completed := moment
			t.Completed = &completed
		}
		times[task] = t
	}
	return nil
}

func applyTaskTimes(t *model.Task, times map[model.ID]activityTimes) {
	at, ok := times[t.ID]
	if !ok || at.Created.IsZero() {
		return
	}
	created, updated := at.Created, at.Updated
	t.CreatedAt, t.UpdatedAt = &created, &updated
	if t.Phase == "done" && at.Completed != nil {
		completed := *at.Completed
		t.CompletedAt = &completed
	}
}

// projectCreatedAt is the time of the project's creation event, which is
// recorded in the owner scope.
func projectCreatedAt(tx readTx, id model.ID) (time.Time, error) {
	var at string
	e := tx.QueryRow("SELECT "+eventTimeSQL+" FROM events WHERE scope='owner' AND json_extract(body,'$.kind')='project.create' AND json_extract(body,'$.resourceId')=? ORDER BY seq LIMIT 1", id).Scan(&at)
	if e == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return parseEventTime(at), e
}

// cachedTimes is a project's task times with every event up to seq folded in.
// A cached map is never written again; a fold works on a copy.
type cachedTimes struct {
	seq   int64
	times map[model.ID]activityTimes
}

// taskTimes serves a project's task times, folding only events newer than the
// cached ones; events are append-only, so the cache never needs repair. The
// returned map is shared and read-only.
func (s *Service) taskTimes(tx readTx, projectID model.ID) (map[model.ID]activityTimes, error) {
	var last int64
	if e := tx.QueryRow("SELECT coalesce(max(seq),0) FROM events WHERE scope=?", projectID).Scan(&last); e != nil {
		return nil, e
	}
	s.timesMu.Lock()
	cached, ok := s.timesCache[projectID]
	s.timesMu.Unlock()
	if ok && cached.seq == last {
		return cached.times, nil
	}
	// A read transaction older than the cache starts over rather than see later events.
	from, times := int64(0), map[model.ID]activityTimes{}
	if ok && cached.seq < last {
		from = cached.seq
		for id, t := range cached.times {
			times[id] = t
		}
	}
	if e := foldTaskTimes(tx, projectID, from, times); e != nil {
		return nil, e
	}
	s.timesMu.Lock()
	if current, ok := s.timesCache[projectID]; !ok || current.seq < last {
		s.timesCache[projectID] = cachedTimes{seq: last, times: times}
	}
	s.timesMu.Unlock()
	return times, nil
}
