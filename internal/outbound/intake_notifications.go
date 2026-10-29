package outbound

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

// Intake has no Task, so its notifications use its own explicit subject.
func EnqueueIntakeNotifications(tx *store.Tx, serviceID, generation string, id model.ID) error {
	var raw string
	if e := tx.QueryRow("SELECT body FROM work_requests WHERE id=?", id).Scan(&raw); e != nil {
		return e
	}
	var intake model.WorkRequest
	if e := json.Unmarshal([]byte(raw), &intake); e != nil {
		return e
	}
	if intake.State == "cancelled" {
		return nil
	}
	if intake.TaskID != "" {
		return EnqueueNotifications(tx, serviceID, generation, intake.TaskID)
	}
	rows, e := tx.Query("SELECT id,version,body FROM decision_requests WHERE request_id=? AND state='pending' ORDER BY id", id)
	if e != nil {
		return e
	}
	notifications := []Notification{}
	for rows.Next() {
		var qid model.ID
		var version int64
		var body string
		if e = rows.Scan(&qid, &version, &body); e != nil {
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
		n, err := BuildNotification(qid, version, kind)
		if err != nil {
			rows.Close()
			return err
		}
		n.IntakeID = id
		n.Text += "\n摘要：" + short(r.Question)
		notifications = append(notifications, n)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if len(notifications) == 0 {
		return nil
	}
	return enqueueNotificationJobs(tx, serviceID, generation, "", id, intake.ProjectID, intake.Text, intake.Version, notifications)
}
