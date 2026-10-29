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

// taskTimes scans the project's events once (events_scope_seq) and attributes
// each event to the task that owns its resource.
func taskTimes(tx readTx, projectID model.ID) (map[model.ID]activityTimes, error) {
	owners, e := resourceTasks(tx, projectID)
	if e != nil {
		return nil, e
	}
	times := map[model.ID]activityTimes{}
	rows, e := tx.Query("SELECT "+eventTimeSQL+",json_extract(body,'$.resourceId'),json_extract(body,'$.kind') FROM events WHERE scope=? ORDER BY seq", projectID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var at string
		var resource, kind sql.NullString
		if e = rows.Scan(&at, &resource, &kind); e != nil {
			return nil, e
		}
		task, ok := owners[model.ID(resource.String)]
		if !ok {
			continue
		}
		moment := parseEventTime(at)
		t := times[task]
		if t.Created.IsZero() || moment.Before(t.Created) {
			t.Created = moment
		}
		if moment.After(t.Updated) {
			t.Updated = moment
		}
		if kind.String == "task.complete" && model.ID(resource.String) == task {
			completed := moment
			t.Completed = &completed
		}
		times[task] = t
	}
	return times, rows.Err()
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
