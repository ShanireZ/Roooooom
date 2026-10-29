package coordinator_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
)

func readJSON(t *testing.T, f *testkit.Fixture, a model.Actor, q model.Query, target any) model.View {
	t.Helper()
	view, e := f.Core.Read(context.Background(), a, q)
	if e != nil {
		t.Fatalf("%s: %v", q.Kind, e)
	}
	if e = json.Unmarshal(view.Data, target); e != nil {
		t.Fatalf("%s: %v", q.Kind, e)
	}
	return view
}

// eventTime is the service time of the first or last event about a resource.
func eventTime(t *testing.T, f *testkit.Fixture, resource model.ID, last bool) time.Time {
	t.Helper()
	order := "ASC"
	if last {
		order = "DESC"
	}
	var raw string
	if e := f.Store.QueryRow(context.Background(), "SELECT coalesce(json_extract(body,'$.receivedAt'),received_at) FROM events WHERE json_extract(body,'$.resourceId')=? ORDER BY seq "+order+" LIMIT 1", resource).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	parsed, e := time.Parse(time.RFC3339Nano, raw)
	if e != nil {
		t.Fatal(e)
	}
	return parsed
}

func projectOf(t *testing.T, f *testkit.Fixture, task model.ID) model.ID {
	t.Helper()
	return f.Task(task).ProjectID
}

func TestTaskTimesFromEvents(t *testing.T) {
	c := makeCandidate(t, "human")
	f := c.F
	p := projectOf(t, f, c.TaskID)
	var tasks []struct {
		ID        model.ID   `json:"id"`
		CreatedAt time.Time  `json:"createdAt"`
		UpdatedAt time.Time  `json:"updatedAt"`
		Completed *time.Time `json:"completedAt"`
	}
	readJSON(t, f, f.Owner(), model.Query{Kind: "tasks", ProjectID: p}, &tasks)
	if len(tasks) != 1 {
		t.Fatalf("tasks %+v", tasks)
	}
	if want := eventTime(t, f, c.TaskID, false); !tasks[0].CreatedAt.Equal(want) {
		t.Fatalf("createdAt %v; want %v", tasks[0].CreatedAt, want)
	}
	if want := eventTime(t, f, c.CandidateID, true); !tasks[0].UpdatedAt.Equal(want) {
		t.Fatalf("updatedAt %v; want the candidate event %v", tasks[0].UpdatedAt, want)
	}
	if delivery := eventTime(t, f, c.DeliveryID, true); tasks[0].UpdatedAt.Before(delivery) {
		t.Fatal("updatedAt is older than the delivery")
	}
	if tasks[0].Completed != nil {
		t.Fatal("open task reports completedAt")
	}
	var one struct {
		CreatedAt time.Time `json:"createdAt"`
	}
	readJSON(t, f, f.Owner(), model.Query{Kind: "task", TargetID: c.TaskID}, &one)
	if !one.CreatedAt.Equal(tasks[0].CreatedAt) {
		t.Fatal("single task view lacks the same createdAt")
	}
	var projects []struct {
		CreatedAt time.Time `json:"createdAt"`
	}
	readJSON(t, f, f.Owner(), model.Query{Kind: "projects"}, &projects)
	if len(projects) != 1 || !projects[0].CreatedAt.Equal(eventTime(t, f, p, false)) {
		t.Fatalf("project createdAt %+v", projects)
	}
}

func TestTaskBodyNeverPersistsTimes(t *testing.T) {
	c := makeCandidate(t, "human")
	f := c.F
	var tasks []map[string]any
	readJSON(t, f, f.Owner(), model.Query{Kind: "tasks", ProjectID: projectOf(t, f, c.TaskID)}, &tasks)
	task := f.Task(c.TaskID)
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = task.Version
	f.Must(f.Owner(), "task.cancel", c.TaskID, map[string]any{"reason": "persist check"}, meta)
	var n int
	if e := f.Store.QueryRow(context.Background(), "SELECT count(*) FROM tasks WHERE json_extract(body,'$.createdAt') IS NOT NULL OR json_extract(body,'$.updatedAt') IS NOT NULL").Scan(&n); e != nil || n != 0 {
		t.Fatalf("task bodies persisted view times: %d, %v", n, e)
	}
}

func TestAssignmentsByProject(t *testing.T) {
	c := makeCandidate(t, "human")
	f := c.F
	p := projectOf(t, f, c.TaskID)
	var rows []struct {
		ID     model.ID `json:"id"`
		TaskID model.ID `json:"taskId"`
	}
	readJSON(t, f, c.Agent, model.Query{Kind: "assignments", ProjectID: p}, &rows)
	if len(rows) != 1 || rows[0].ID != c.AssignmentID || rows[0].TaskID != c.TaskID {
		t.Fatalf("project assignments %+v", rows)
	}
	task := f.Task(c.TaskID)
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = task.Version
	f.Must(f.Owner(), "task.cancel", c.TaskID, map[string]any{"reason": "closed task leaves the room"}, meta)
	readJSON(t, f, f.Owner(), model.Query{Kind: "assignments", ProjectID: p}, &rows)
	if len(rows) != 0 {
		t.Fatalf("cancelled task assignments still listed %+v", rows)
	}
}

type needsYouItem struct {
	Kind              string    `json:"kind"`
	ProjectID         model.ID  `json:"projectId"`
	TaskID            model.ID  `json:"taskId"`
	RequestID         model.ID  `json:"requestId"`
	DecisionRequestID model.ID  `json:"decisionRequestId"`
	CandidateID       model.ID  `json:"candidateId"`
	Code              string    `json:"code"`
	Summary           string    `json:"summary"`
	Since             time.Time `json:"since"`
}

func needsYou(t *testing.T, f *testkit.Fixture) []needsYouItem {
	t.Helper()
	var items []needsYouItem
	readJSON(t, f, f.Owner(), model.Query{Kind: "needs-you"}, &items)
	return items
}

func find(items []needsYouItem, kind string, match func(needsYouItem) bool) *needsYouItem {
	for i := range items {
		if items[i].Kind == kind && match(items[i]) {
			return &items[i]
		}
	}
	return nil
}

func TestNeedsYouKinds(t *testing.T) {
	c := makeCandidate(t, "human")
	f := c.F
	p := projectOf(t, f, c.TaskID)
	acceptance := find(needsYou(t, f), "acceptance", func(i needsYouItem) bool { return i.TaskID == c.TaskID })
	if acceptance == nil || acceptance.CandidateID != c.CandidateID || acceptance.ProjectID != p || acceptance.Since.IsZero() || acceptance.Summary == "" {
		t.Fatalf("ready acceptance missing: %+v", acceptance)
	}

	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	risky := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	decision := find(needsYou(t, f), "decision", func(i needsYouItem) bool { return i.TaskID == risky.ResourceID })
	if decision == nil || decision.DecisionRequestID == "" || decision.Summary == "" || decision.Since.IsZero() {
		t.Fatalf("pending decision missing: %+v", decision)
	}

	task := f.Task(c.TaskID)
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = task.Version
	meta.IntegratorEpoch = task.IntegratorEpoch
	meta.AcceptancePolicyVersion = task.AcceptancePolicyVersion
	f.Must(f.Owner(), "acceptance.accept", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta)
	if find(needsYou(t, f), "acceptance", func(i needsYouItem) bool { return i.TaskID == c.TaskID }) != nil {
		t.Fatal("already accepted candidate still listed")
	}

	if e := f.Identities.Revoke(context.Background(), f.Owner(), c.Agent.ParticipantID); e != nil {
		t.Fatal(e)
	}
	if find(needsYou(t, f), "blocker", func(i needsYouItem) bool { return i.TaskID == c.TaskID && i.Code == "integrator-revoked" }) == nil {
		t.Fatalf("revoked integrator blocker missing: %+v", needsYou(t, f))
	}
}

func TestNeedsYouExcludesStaleCandidate(t *testing.T) {
	c := makeCandidate(t, "human")
	f := c.F
	task := f.Task(c.TaskID)
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = task.Version
	meta.IntegratorEpoch = task.IntegratorEpoch
	meta.AcceptancePolicyVersion = task.AcceptancePolicyVersion
	policy := task.AcceptancePolicy
	policy.Conditions = append(policy.Conditions, "new owner requirement")
	f.Must(f.Owner(), "acceptance.policy", c.TaskID, map[string]any{"policy": policy, "reason": "new requirement"}, meta)
	if item := find(needsYou(t, f), "acceptance", func(i needsYouItem) bool { return i.TaskID == c.TaskID }); item != nil {
		t.Fatalf("stale candidate listed: %+v", item)
	}
}

func TestNeedsYouEndedPreparer(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "preparer")
	id := createIntake(f, p)
	f.Must(a, "request.claim", id, map[string]any{}, intakeMeta(f, a, intake(t, f, a, id)))
	m := f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "session.end", a.SessionID, map[string]any{"summary": "ended"}, m)
	if find(needsYou(t, f), "blocker", func(i needsYouItem) bool { return i.RequestID == id && i.Code == "preparer-session-ended" }) == nil {
		t.Fatalf("ended preparer blocker missing: %+v", needsYou(t, f))
	}
}

func TestNeedsYouOwnerOnly(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "agent")
	for _, q := range []model.Query{{Kind: "needs-you"}, {Kind: "activity", ProjectID: p}} {
		if _, e := f.Core.Read(context.Background(), a, q); model.Status(e) != 403 {
			t.Fatalf("agent read %s: %v", q.Kind, e)
		}
	}
}

func TestActivityPagingNewestFirst(t *testing.T) {
	c := makeCandidate(t, "human")
	f := c.F
	p := projectOf(t, f, c.TaskID)
	type entry struct {
		Seq        int64    `json:"seq"`
		Kind       string   `json:"kind"`
		ResourceID model.ID `json:"resourceId"`
		TaskID     model.ID `json:"taskId"`
	}
	var all []entry
	readJSON(t, f, f.Owner(), model.Query{Kind: "activity", ProjectID: p, Limit: 200}, &all)
	if len(all) < 5 {
		t.Fatalf("activity too short: %+v", all)
	}
	for i := 1; i < len(all); i++ {
		if all[i].Seq >= all[i-1].Seq {
			t.Fatalf("activity not newest first at %d: %+v", i, all)
		}
	}
	if all[0].Kind != "acceptance.candidate" || all[0].TaskID != c.TaskID {
		t.Fatalf("newest entry %+v", all[0])
	}
	var page []entry
	view := readJSON(t, f, f.Owner(), model.Query{Kind: "activity", ProjectID: p, Limit: 2}, &page)
	if len(page) != 2 || !view.HasMore || view.Cursor == "" {
		t.Fatalf("first page %+v more=%v", page, view.HasMore)
	}
	var next []entry
	readJSON(t, f, f.Owner(), model.Query{Kind: "activity", ProjectID: p, Limit: 2, Cursor: view.Cursor}, &next)
	if len(next) != 2 || next[0].Seq != all[2].Seq {
		t.Fatalf("second page %+v; want from %+v", next, all[2])
	}
}
