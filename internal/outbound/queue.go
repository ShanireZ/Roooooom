package outbound

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Result struct {
	State     string    `json:"state"`
	RemoteID  string    `json:"remoteId,omitempty"`
	RemoteURL string    `json:"remoteUrl,omitempty"`
	RetryAt   time.Time `json:"retryAt,omitempty"`
	ErrorCode string    `json:"errorCode,omitempty"`
}
type Job struct {
	IntakeID         model.ID      `json:"intakeId,omitempty"`
	OutcomeUncertain bool          `json:"outcomeUncertain"`
	Notification     *Notification `json:"notification,omitempty"`
	ID               model.ID      `json:"jobId"`
	TaskID           model.ID      `json:"taskId"`
	BindingID        model.ID      `json:"bindingId"`
	Version          int64         `json:"version"`
	Kind             string        `json:"kind"`
	Status           string        `json:"status"`
	Marker           string        `json:"marker"`
	Title            string        `json:"title"`
	Text             string        `json:"text"`
	IssueNumber      string        `json:"issueNumber,omitempty"`
	State            string        `json:"state,omitempty"`
	ErrorCode        string        `json:"errorCode,omitempty"`
	Attempts         int           `json:"attempts"`
	SourceEventID    int64         `json:"sourceEventId"`
	TaskVersion      int64         `json:"taskVersion"`
	Binding          Binding       `json:"-"`
}
type Adapter interface {
	Deliver(context.Context, Job) (Result, error)
	Reconcile(context.Context, Job) (Result, error)
}
type Queue struct {
	st            *store.Store
	adapters      map[string]Adapter
	mu            sync.Mutex
	suppressAfter int64
}

func NewQueue(st *store.Store, adapters map[string]Adapter) *Queue {
	return &Queue{st: st, adapters: adapters}
}
func (q *Queue) Recover(ctx context.Context) error {
	_, e := q.st.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("UPDATE external_jobs SET status='uncertain',version=version+1 WHERE status='sending'")
		return model.Receipt{}, e
	})
	return e
}
func (q *Queue) Run(ctx context.Context) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-timer.C:
			_ = q.RunOnce(ctx, now)
		}
	}
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

// Enqueue runs inside the domain mutation transaction. It projects allowlisted
// metadata only: never the event body, raw chat, credentials, or remote input.
func Enqueue(tx *store.Tx, serviceID, generation, kind string, resource, project model.ID, seq int64) error {
	if project == "owner" || strings.HasPrefix(kind, "message.") || strings.HasPrefix(kind, "receipt.") {
		return nil
	}
	taskID := resource
	lookup := ""
	switch kind {
	case "integration.record":
		lookup = "SELECT task_id FROM integrations WHERE id=?"
	case "decision.request", "decision.resolve", "decision.supersede":
		lookup = "SELECT COALESCE(task_id,(SELECT task_id FROM work_requests WHERE id=decision_requests.request_id),'') FROM decision_requests WHERE id=?"
	case "delivery.submit":
		lookup = "SELECT a.task_id FROM assignments a JOIN deliveries d ON d.assignment_id=a.id WHERE d.id=?"
	case "delivery.review":
		lookup = "SELECT task_id FROM assignments WHERE id=?"
	case "acceptance.candidate":
		lookup = "SELECT task_id FROM acceptance_candidates WHERE id=?"
	case "acceptance.accept":
		lookup = "SELECT task_id FROM acceptances WHERE id=?"
	}
	if lookup != "" {
		if e := tx.QueryRow(lookup, resource).Scan(&taskID); e != nil {
			return e
		}
	}
	if taskID == "" {
		return nil
	}
	switch kind {
	case "task.create", "task.complete", "task.cancel", "task.reopen", "integration.record", "decision.request", "decision.resolve", "decision.supersede", "delivery.submit", "delivery.review", "acceptance.candidate", "acceptance.accept":
	default:
		return nil
	}
	var raw string
	if e := tx.QueryRow("SELECT body FROM tasks WHERE id=?", taskID).Scan(&raw); e != nil {
		return e
	}
	var task model.Task
	if e := json.Unmarshal([]byte(raw), &task); e != nil {
		return e
	}
	rows, e := tx.Query("SELECT body FROM bindings WHERE project_id=? AND kind='github' ORDER BY id", project)
	if e != nil {
		return e
	}
	bindings := []Binding{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return e
		}
		var b Binding
		if e = json.Unmarshal([]byte(raw), &b); e != nil {
			rows.Close()
			return e
		}
		bindings = append(bindings, b)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, b := range bindings {
		if !b.Enabled || b.NeedsRebind || !contains(b.Events, kind) {
			continue
		}
		prefix := serviceID + ":" + generation + ":" + string(b.ID) + ":" + string(taskID)
		text := "Roooooom local task record. Local dashboard is authoritative.\n"
		title := "Roooooom task " + string(taskID)
		if contains(b.Fields, "taskId") {
			text += "Task: " + string(taskID) + "\n"
		}
		if contains(b.Fields, "goal") {
			goal := short(task.Goal)
			title = goal
			if len([]rune(title)) > 180 {
				title = string([]rune(title)[:180])
			}
			text += "Goal: " + goal + "\n"
		}
		if contains(b.Fields, "state") {
			text += "State: " + task.Phase + "\n"
		}
		if contains(b.Fields, "scope") {
			text += "Scope: " + summaryList(task.Scope) + "\n"
		}
		if contains(b.Fields, "acceptance") {
			text += "Acceptance: " + summaryList(task.AcceptancePolicy.Conditions) + "\n"
		}
		text = truncateText(text, 24*1024) + "\nTask version: " + string(model.JSON(task.Version)) + "; source event: " + string(model.JSON(seq)) + "\nOpen the local Roooooom dashboard to find this task.\n"
		issue := Job{ID: model.ID(digest(prefix + ":issue")), TaskID: taskID, BindingID: b.ID, Version: 1, Kind: "github.issue", Status: "queued", Marker: "<!-- roooooom:" + prefix + " -->", Title: title, Text: text}
		issue.SourceEventID = seq
		issue.TaskVersion = task.Version
		if e = insertJob(tx, issue, prefix+":issue"); e != nil {
			return e
		}
		if kind != "task.create" {
			j := issue
			j.Kind = "github.comment"
			j.ID = model.ID(digest(prefix + ":" + string(model.JSON(seq))))
			j.Marker = "<!-- roooooom:job:" + string(j.ID) + " -->"
			j.Text = text + "Milestone: " + kind + "\n"
			if e = insertJob(tx, j, string(j.ID)); e != nil {
				return e
			}
			if kind == "task.complete" || kind == "task.cancel" || kind == "task.reopen" {
				j.Kind = "github.state"
				j.ID = model.ID(digest(string(j.ID) + ":state"))
				j.State = "closed"
				if kind == "task.reopen" {
					j.State = "open"
				}
				if e = insertJob(tx, j, string(j.ID)); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func insertJob(tx *store.Tx, j Job, key string) error {
	var task, request any
	if j.TaskID != "" {
		task = j.TaskID
	} else if j.IntakeID != "" {
		request = j.IntakeID
	}
	_, e := tx.Exec("INSERT OR IGNORE INTO external_jobs(id,binding_id,task_id,version,kind,status,dedupe_key,body,request_id) VALUES(?,?,?,?,?,?,?,?,?)", j.ID, j.BindingID, task, j.Version, j.Kind, j.Status, key, string(model.JSON(j)), request)
	return e
}
func summaryList(values []string) string {
	list := []string{}
	for i, v := range values {
		if i == 20 {
			list = append(list, "… additional entries omitted")
			break
		}
		list = append(list, short(v))
	}
	return strings.Join(list, "; ")
}
func truncateText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	suffix := "… summary truncated"
	limit -= len(suffix)
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit] + suffix
}
func (q *Queue) RunOnce(ctx context.Context, now time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	var job Job
	var previous string
	var suppressAfter int64
	_, e := q.st.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var err error
		suppressAfter, err = suppressStaleNotifications(tx, q.suppressAfter)
		if err != nil {
			return model.Receipt{}, err
		}
		var raw, braw string
		e := tx.QueryRow(`SELECT j.body,b.body,j.status,j.version FROM external_jobs j JOIN bindings b ON b.id=j.binding_id LEFT JOIN outbound_channels g ON g.channel_key=json_extract(b.body,'$.channelId') WHERE j.status IN('queued','retry','uncertain') AND (j.retry_at IS NULL OR j.retry_at<=?) AND (g.next_at IS NULL OR g.next_at<=?) AND json_extract(b.body,'$.enabled')=1 AND coalesce(json_extract(b.body,'$.needsRebind'),0)=0 AND NOT EXISTS(SELECT 1 FROM external_jobs prev WHERE prev.binding_id=j.binding_id AND prev.rowid<j.rowid AND prev.status NOT IN('delivered','canceled','accepted')) ORDER BY j.rowid LIMIT 1`, now.UTC().Format(time.RFC3339Nano), now.UTC().Format(time.RFC3339Nano)).Scan(&raw, &braw, &previous, &job.Version)
		if errors.Is(e, sql.ErrNoRows) {
			return model.Receipt{}, nil
		}
		if e != nil {
			return model.Receipt{}, e
		}
		version := job.Version
		if e = json.Unmarshal([]byte(raw), &job); e != nil {
			return model.Receipt{}, e
		}
		job.Version = version
		if previous == "uncertain" {
			job.OutcomeUncertain = true
		}
		if e = json.Unmarshal([]byte(braw), &job.Binding); e != nil {
			return model.Receipt{}, e
		}
		if q.adapters[job.Binding.Kind] == nil {
			job = Job{}
			return model.Receipt{}, nil
		}
		if job.Binding.Kind == "dingtalk" {
			current, e := NotificationCurrent(tx, job)
			if e != nil {
				return model.Receipt{}, e
			}
			if !current {
				_, e = tx.Exec("UPDATE external_jobs SET status='canceled',version=version+1 WHERE id=?", job.ID)
				if e == nil {
					_, e = tx.AppendEvent("owner", model.JSON(map[string]any{"kind": "outbound.suppressed", "resourceId": job.ID}))
				}
				job = Job{}
				return model.Receipt{}, e
			}
		}
		if job.Kind != "github.issue" && job.Binding.Kind == "github" {
			var receiptRaw string
			if e = tx.QueryRow("SELECT r.body FROM external_receipts r JOIN external_jobs p ON p.id=r.job_id WHERE p.binding_id=? AND p.task_id=? AND p.kind='github.issue'", job.BindingID, job.TaskID).Scan(&receiptRaw); e != nil {
				return model.Receipt{}, e
			}
			var result Result
			if e = json.Unmarshal([]byte(receiptRaw), &result); e != nil {
				return model.Receipt{}, e
			}
			job.IssueNumber = result.RemoteID
		}
		job.Attempts++
		job.Version++
		job.Status = "sending"
		interval := time.Second
		if job.Binding.Kind == "dingtalk" {
			interval = 6 * time.Second
		}
		_, e = tx.Exec("INSERT INTO outbound_channels(channel_key,next_at) VALUES(?,?) ON CONFLICT(channel_key) DO UPDATE SET next_at=excluded.next_at", job.Binding.ChannelID, now.Add(interval).UTC().Format(time.RFC3339Nano))
		if e != nil {
			return model.Receipt{}, e
		}
		_, e = tx.Exec("UPDATE external_jobs SET status='sending',version=?,body=? WHERE id=?", job.Version, string(model.JSON(job)), job.ID)
		return model.Receipt{}, e
	})
	if e == nil {
		q.suppressAfter = suppressAfter
	}
	if e != nil || job.ID == "" {
		return e
	}
	netctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	a := q.adapters[job.Binding.Kind]
	var result Result
	if previous == "uncertain" || job.OutcomeUncertain {
		result, e = a.Reconcile(netctx, job)
	} else {
		result, e = a.Deliver(netctx, job)
	}
	if e != nil {
		result = Result{State: "uncertain", ErrorCode: "transport-outcome-unknown"}
	}
	switch result.State {
	case "delivered", "retry", "uncertain", "paused", "accepted":
	default:
		result = Result{State: "uncertain", ErrorCode: "invalid-adapter-result"}
	}
	if result.State == "uncertain" {
		job.OutcomeUncertain = true
	}
	if job.Attempts >= 8 && result.State == "retry" {
		result.State = "paused"
		result.ErrorCode = "retry-budget-exhausted"
	}
	if job.Binding.Kind == "dingtalk" && (result.State == "retry" || result.State == "uncertain") {
		delays := []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute}
		index := job.Attempts - 1
		if index > 2 {
			index = 2
		}
		if result.RetryAt.Before(now.Add(delays[index])) {
			result.RetryAt = now.Add(delays[index])
		}
		if job.Attempts >= 3 {
			result.State = "paused"
			result.ErrorCode = "notification-attempt-budget-exhausted"
		}
	}
	if result.RetryAt.Before(now.Add(time.Second)) {
		result.RetryAt = now.Add(time.Second)
	}
	if result.State == "uncertain" && job.Binding.Kind != "dingtalk" && result.RetryAt.Before(now.Add(time.Minute)) {
		result.RetryAt = now.Add(time.Minute)
	}
	_, e = q.st.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var enabled bool
		if e := tx.QueryRow("SELECT json_extract(body,'$.enabled') FROM bindings WHERE id=?", job.BindingID).Scan(&enabled); e != nil {
			return model.Receipt{}, e
		}
		job.Status = result.State
		job.ErrorCode = result.ErrorCode
		job.Version++
		// A result still belongs to its immutable original binding even if disabled.
		if _, e := tx.Exec("UPDATE external_jobs SET status=?,version=?,retry_at=?,body=? WHERE id=? AND status='sending'", result.State, job.Version, result.RetryAt.UTC().Format(time.RFC3339Nano), string(model.JSON(job)), job.ID); e != nil {
			return model.Receipt{}, e
		}
		if _, e := tx.Exec("INSERT INTO outbound_channels(channel_key,next_at) VALUES(?,?) ON CONFLICT(channel_key) DO UPDATE SET next_at=max(next_at,excluded.next_at)", job.Binding.ChannelID, result.RetryAt.UTC().Format(time.RFC3339Nano)); e != nil {
			return model.Receipt{}, e
		}
		if result.State == "delivered" || result.State == "accepted" {
			if _, e := tx.Exec("INSERT OR REPLACE INTO external_receipts(job_id,body) VALUES(?,?)", job.ID, string(model.JSON(result))); e != nil {
				return model.Receipt{}, e
			}
		}
		_, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": "outbound.status", "resourceId": job.ID, "state": result.State, "errorCode": result.ErrorCode}))
		return model.Receipt{}, e
	})
	return e
}
