package outbound

import (
	"encoding/json"
	"regexp"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"strconv"
	"strings"
)

type Notification struct {
	IntakeID  model.ID `json:"intakeId,omitempty"`
	ID        string   `json:"id"`
	RequestID model.ID `json:"requestId"`
	Version   int64    `json:"version"`
	Kind      string   `json:"kind"`
	TaskID    model.ID `json:"taskId"`
	Text      string   `json:"text"`
}

var links = regexp.MustCompile(`(?i)https?://[^\s]+`)

func short(s string) string {
	s = links.ReplaceAllString(Redact(s), "[reference omitted]")
	s = strings.ReplaceAll(s, "@", "＠")
	r := []rune(s)
	if len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}
func BuildNotification(requestID model.ID, version int64, kind string) (Notification, error) {
	action := ""
	switch kind {
	case "decision-pending":
		action = "有待你拍板的事項"
	case "human-acceptance":
		action = "有指定由你驗收的候選"
	case "owner-blocker":
		action = "有需要你處理的阻塞"
	default:
		return Notification{}, model.Invalid("unsupported notification kind")
	}
	if !safeRef.MatchString(string(requestID)) || version < 1 {
		return Notification{}, model.Invalid("request and version required")
	}
	return Notification{ID: digest(string(requestID) + ":" + strconv.FormatInt(version, 10) + ":" + kind), RequestID: requestID, Version: version, Kind: kind, Text: "Roooooom · " + action + "\nRequest: " + string(requestID) + " · v" + strconv.FormatInt(version, 10) + "\n請回到這台電腦的 Roooooom 決策頁處理；手機只能收提醒。"}, nil
}
func EnqueueNotifications(tx *store.Tx, serviceID, generation string, taskID model.ID) error {
	var raw string
	if e := tx.QueryRow("SELECT body FROM tasks WHERE id=?", taskID).Scan(&raw); e != nil {
		return e
	}
	var task model.Task
	if e := json.Unmarshal([]byte(raw), &task); e != nil {
		return e
	}
	if task.Phase == "done" || task.Phase == "cancelled" {
		return nil
	}
	notifications := []Notification{}
	rows, e := tx.Query("SELECT id,version,body FROM decision_requests WHERE (task_id=? OR request_id IN(SELECT id FROM work_requests WHERE task_id=?)) AND state='pending' ORDER BY id", taskID, taskID)
	if e != nil {
		return e
	}
	for rows.Next() {
		var id model.ID
		var version int64
		var body string
		if e = rows.Scan(&id, &version, &body); e != nil {
			rows.Close()
			return e
		}
		var r struct {
			Question   string `json:"question"`
			ActionKind string `json:"actionKind"`
		}
		if e = json.Unmarshal([]byte(body), &r); e != nil {
			rows.Close()
			return e
		}
		kind := r.ActionKind
		if kind == "" {
			kind = "decision-pending"
		}
		n, e := BuildNotification(id, version, kind)
		if e != nil {
			rows.Close()
			return e
		}
		n.TaskID = taskID
		n.Text += "\n摘要：" + short(r.Question)
		notifications = append(notifications, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if task.CurrentCandidateID != "" && task.AcceptancePolicy.Mode == "human" {
		var n int
		if e = tx.QueryRow("SELECT count(*) FROM acceptances WHERE candidate_id=? AND json_extract(body,'$.actorKind')='human'", task.CurrentCandidateID).Scan(&n); e != nil {
			return e
		}
		if n == 0 {
			notification, e := BuildNotification(task.CurrentCandidateID, 1, "human-acceptance")
			if e != nil {
				return e
			}
			notification.TaskID = taskID
			notifications = append(notifications, notification)
		}
	}
	if len(notifications) == 0 {
		return nil
	}
	return enqueueNotificationJobs(tx, serviceID, generation, taskID, "", task.ProjectID, task.Goal, task.Version, notifications)
}
func enqueueNotificationJobs(tx *store.Tx, serviceID, generation string, taskID, intakeID, projectID model.ID, title string, version int64, notifications []Notification) error {
	rows, e := tx.Query("SELECT body FROM bindings WHERE project_id=? AND kind='dingtalk'", projectID)
	if e != nil {
		return e
	}
	bindings := []Binding{}
	for rows.Next() {
		var body string
		if e = rows.Scan(&body); e != nil {
			rows.Close()
			return e
		}
		var b Binding
		if e = json.Unmarshal([]byte(body), &b); e != nil {
			rows.Close()
			return e
		}
		if b.Enabled && !b.NeedsRebind {
			bindings = append(bindings, b)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, b := range bindings {
		for _, n := range notifications {
			if !contains(b.Events, n.Kind) {
				continue
			}
			if !contains(b.Fields, "summary") {
				parts := strings.Split(n.Text, "\n摘要：")
				n.Text = parts[0]
			}
			if contains(b.Fields, "goal") {
				label := "任務："
				if intakeID != "" {
					label = "需求："
				}
				n.Text += "\n" + label + short(title)
			}
			if contains(b.Fields, "projectTitle") {
				var raw string
				if e = tx.QueryRow("SELECT body FROM projects WHERE id=?", projectID).Scan(&raw); e != nil {
					return e
				}
				var p struct {
					Name string `json:"name"`
				}
				if e = json.Unmarshal([]byte(raw), &p); e != nil {
					return e
				}
				n.Text += "\n專案：" + short(p.Name)
			}
			if contains(b.Fields, "taskId") && taskID != "" {
				n.Text += "\nTask: " + string(taskID)
			}
			id := model.ID(digest(serviceID + ":" + generation + ":" + b.ChannelID + ":" + n.ID))
			job := Job{ID: id, TaskID: taskID, IntakeID: intakeID, BindingID: b.ID, Version: 1, Kind: "dingtalk.notify", Status: "queued", Text: n.Text, Notification: &n, TaskVersion: version}
			if e = insertJob(tx, job, string(id)); e != nil {
				return e
			}
		}
	}
	return nil
}
func NotificationCurrent(tx *store.Tx, j Job) (bool, error) {
	n := j.Notification
	if n == nil {
		return false, nil
	}
	if j.IntakeID != "" && j.TaskID == "" {
		var state string
		if e := tx.QueryRow("SELECT state FROM work_requests WHERE id=?", j.IntakeID).Scan(&state); e != nil {
			return false, e
		}
		if state == "cancelled" || state == "converted" {
			return false, nil
		}
		var count int
		e := tx.QueryRow("SELECT count(*) FROM decision_requests WHERE id=? AND request_id=? AND version=? AND state='pending'", n.RequestID, j.IntakeID, n.Version).Scan(&count)
		return count == 1, e
	}
	var raw string
	if e := tx.QueryRow("SELECT body FROM tasks WHERE id=?", j.TaskID).Scan(&raw); e != nil {
		return false, e
	}
	var task model.Task
	if e := json.Unmarshal([]byte(raw), &task); e != nil {
		return false, e
	}
	if task.Phase == "done" || task.Phase == "cancelled" {
		return false, nil
	}
	if n.Kind == "human-acceptance" {
		var accepted int
		if e := tx.QueryRow("SELECT count(*) FROM acceptances WHERE candidate_id=? AND json_extract(body,'$.actorKind')='human'", n.RequestID).Scan(&accepted); e != nil {
			return false, e
		}
		return task.AcceptancePolicy.Mode == "human" && task.CurrentCandidateID == n.RequestID && accepted == 0, nil
	}
	var count int
	e := tx.QueryRow("SELECT count(*) FROM decision_requests WHERE id=? AND (task_id=? OR request_id IN(SELECT id FROM work_requests WHERE task_id=?)) AND version=? AND state='pending'", n.RequestID, j.TaskID, j.TaskID, n.Version).Scan(&count)
	return count == 1, e
}

// Rotate a bounded batch so paused or backoff entries are also suppressed.
// Terminal history stays untouched; a process restart simply starts the scan again.
func suppressStaleNotifications(tx *store.Tx, after int64) (int64, error) {
	rows, e := tx.Query("SELECT rowid,body FROM external_jobs WHERE kind='dingtalk.notify' AND status IN('queued','retry','uncertain','paused') AND rowid>? ORDER BY rowid LIMIT 100", after)
	if e != nil {
		return after, e
	}
	type entry struct {
		row int64
		job Job
	}
	entries := []entry{}
	for rows.Next() {
		var item entry
		var raw string
		if e = rows.Scan(&item.row, &raw); e != nil {
			rows.Close()
			return after, e
		}
		if e = json.Unmarshal([]byte(raw), &item.job); e != nil {
			rows.Close()
			return after, e
		}
		entries = append(entries, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return after, e
	}
	if len(entries) == 0 {
		return 0, nil
	}
	for _, item := range entries {
		current, e := NotificationCurrent(tx, item.job)
		if e != nil {
			return after, e
		}
		if !current {
			if _, e = tx.Exec("UPDATE external_jobs SET status='canceled',version=version+1 WHERE id=?", item.job.ID); e != nil {
				return after, e
			}
			if _, e = tx.AppendEvent("owner", model.JSON(map[string]any{"kind": "outbound.suppressed", "resourceId": item.job.ID})); e != nil {
				return after, e
			}
		}
	}
	return entries[len(entries)-1].row, nil
}
