package coordinator_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
)

type intakeView struct {
	ID                    model.ID `json:"id"`
	ProjectID             model.ID `json:"projectId"`
	Version               int64    `json:"version"`
	Text                  string   `json:"text"`
	State                 string   `json:"state"`
	PreparerParticipantID model.ID `json:"preparerParticipantId"`
	PreparerSessionID     model.ID `json:"preparerSessionId"`
	PreparationEpoch      int64    `json:"preparationEpoch"`
	TaskID                model.ID `json:"taskId"`
	Constraints           []string `json:"constraints"`
}

func intake(t *testing.T, f *testkit.Fixture, actor model.Actor, id model.ID) intakeView {
	t.Helper()
	view, e := f.Core.Read(context.Background(), actor, model.Query{Kind: "request", TargetID: id})
	if e != nil {
		t.Fatal(e)
	}
	var r intakeView
	if e = json.Unmarshal(view.Data, &r); e != nil {
		t.Fatal(e)
	}
	return r
}
func intakeMeta(f *testkit.Fixture, a model.Actor, r intakeView) model.Mutation {
	m := f.Meta(a)
	m.ExpectedVersion = r.Version
	m.ClaimEpoch = r.PreparationEpoch
	return m
}
func createIntake(f *testkit.Fixture, p model.ID) model.ID {
	return f.Must(f.Owner(), "request.create", "", map[string]any{"projectId": p, "text": "做一份可讀的每週摘要", "constraints": []string{"保留登入方式"}}, f.Meta(f.Owner())).ResourceID
}

// Removing the atomic ownership check would allow two agents to prepare/convert the same request.
func TestRequestSingleClaimAndConversion(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "a"), f.Agent(p, "b")
	id := createIntake(f, p)
	r := intake(t, f, f.Owner(), id)
	if r.Text != "做一份可讀的每週摘要" || r.State != "waiting" || f.Count("tasks") != 0 {
		t.Fatal(r)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	type result struct {
		a model.Actor
		e error
	}
	results := make(chan result, 2)
	for _, actor := range []model.Actor{a, b} {
		wg.Go(func() {
			<-start
			_, e := f.Apply(actor, "request.claim", id, map[string]any{}, intakeMeta(f, actor, r))
			results <- result{actor, e}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	wins := 0
	var winner model.Actor
	for item := range results {
		if item.e == nil {
			wins++
			winner = item.a
		} else if model.Status(item.e) != 409 {
			t.Fatal(item.e)
		}
	}
	if wins != 1 {
		t.Fatal("multiple preparations", wins)
	}
	r = intake(t, f, winner, id)
	m := intakeMeta(f, winner, r)
	payload := map[string]any{"task": testkit.TaskPayload(p, 1)}
	receipt := f.Must(winner, "request.convert", id, payload, m)
	retry := f.Must(winner, "request.convert", id, payload, m)
	if receipt.EventID != retry.EventID || f.Count("tasks") != 1 {
		t.Fatal("duplicate conversion")
	}
	r = intake(t, f, winner, id)
	task := f.Task(r.TaskID)
	var source struct {
		SourceRequestID   model.ID `json:"sourceRequestId"`
		SourceRequestText string   `json:"sourceRequestText"`
		SourceConstraints []string `json:"sourceConstraints"`
	}
	view, e := f.Core.Read(context.Background(), f.Owner(), model.Query{Kind: "task", TargetID: r.TaskID})
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(view.Data, &source); e != nil {
		t.Fatal(e)
	}
	if r.State != "converted" || task.IntegratorSessionID != winner.SessionID || source.SourceRequestID != id || source.SourceRequestText != r.Text || len(source.SourceConstraints) != 1 || source.SourceConstraints[0] != "保留登入方式" {
		t.Fatal(r, task, source)
	}
	var created int
	if e = f.Store.QueryRow(context.Background(), "SELECT count(*) FROM events WHERE json_extract(body,'$.kind')='task.create' AND json_extract(body,'$.resourceId')=?", task.ID).Scan(&created); e != nil || created != 1 {
		t.Fatal("missing task-created event", created, e)
	}
}

// Omitting the preparer/session/epoch checks lets an active or unknown session lose work without authorization.
func TestRequestAuthorityAndResume(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "a"), f.Agent(p, "b")
	other, _ := f.Project()
	foreign := f.Agent(other, "foreign")
	id := createIntake(f, p)
	r := intake(t, f, a, id)
	if _, e := f.Apply(a, "request.create", "", map[string]any{"projectId": p, "text": "not human"}, f.Meta(a)); model.Status(e) != 403 {
		t.Fatal("agent created human intake", e)
	}
	if _, e := f.Core.Read(context.Background(), foreign, model.Query{Kind: "request", TargetID: id}); model.Status(e) != 403 {
		t.Fatal("foreign read", e)
	}
	f.Must(a, "request.claim", id, map[string]any{}, intakeMeta(f, a, r))
	r = intake(t, f, a, id)
	if _, e := f.Apply(b, "request.assign", id, map[string]any{"toSessionId": b.SessionID, "reason": "take"}, intakeMeta(f, b, r)); model.Status(e) != 403 {
		t.Fatal("agent forced takeover", e)
	}
	fresh := f.NewSession(a)
	if _, e := f.Apply(fresh, "request.resume", id, map[string]any{"expectedSourceSessionId": a.SessionID}, intakeMeta(f, fresh, r)); model.Status(e) != 409 {
		t.Fatal("active session stolen", e)
	}
	m := f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "session.end", a.SessionID, map[string]any{"summary": "handoff explicitly"}, m)
	f.Must(fresh, "request.resume", id, map[string]any{"expectedSourceSessionId": a.SessionID}, intakeMeta(f, fresh, r))
	r = intake(t, f, fresh, id)
	if r.PreparerSessionID != fresh.SessionID || r.PreparationEpoch != 2 {
		t.Fatal(r)
	}
	old := intakeMeta(f, fresh, r)
	f.Must(fresh, "request.handoff", id, map[string]any{"toSessionId": b.SessionID, "reason": "voluntary"}, old)
	offered := intake(t, f, b, id)
	if offered.PreparerSessionID != fresh.SessionID {
		t.Fatal("offer transferred before acceptance")
	}
	f.Must(b, "request.accept", id, map[string]any{}, intakeMeta(f, b, offered))
	r = intake(t, f, b, id)
	if r.PreparationEpoch != 3 || r.PreparerSessionID != b.SessionID {
		t.Fatal(r)
	}
	if _, e := f.Apply(fresh, "request.release", id, map[string]any{"reason": "late"}, old); model.Status(e) != 409 {
		t.Fatal("old owner changed request", e)
	}
	f.Must(b, "request.release", id, map[string]any{"reason": "return"}, intakeMeta(f, b, r))
	r = intake(t, f, b, id)
	if r.State != "waiting" || r.PreparerSessionID != "" {
		t.Fatal(r)
	}
	f.Must(f.Owner(), "request.assign", id, map[string]any{"toSessionId": fresh.SessionID, "reason": "owner chooses new preparer"}, intakeMeta(f, f.Owner(), r))
	r = intake(t, f, fresh, id)
	if r.State != "preparing" || r.PreparerSessionID != fresh.SessionID {
		t.Fatal("owner takeover not applied", r)
	}
}

// Silently replacing the owner's capacity/policy during conversion loses explicit requirements.
func TestRequestOwnerConstraintsAndCancellation(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "a")
	policy := model.AcceptancePolicy{Mode: "human", Conditions: []string{"人類確認畫面"}}
	id := f.Must(f.Owner(), "request.create", "", map[string]any{"projectId": p, "text": "簡化介面", "constraints": []string{}, "collaboratorLimit": 0, "acceptancePolicy": policy}, f.Meta(f.Owner())).ResourceID
	r := intake(t, f, a, id)
	f.Must(a, "request.claim", id, map[string]any{}, intakeMeta(f, a, r))
	r = intake(t, f, a, id)
	task := testkit.TaskPayload(p, 1)
	task.AcceptancePolicy = policy
	if _, e := f.Apply(a, "request.convert", id, map[string]any{"task": task}, intakeMeta(f, a, r)); model.Status(e) != 403 {
		t.Fatal("owner capacity changed", e)
	}
	task.CollaboratorLimit = 0
	task.AcceptancePolicy = testkit.TaskPayload(p, 0).AcceptancePolicy
	if _, e := f.Apply(a, "request.convert", id, map[string]any{"task": task}, intakeMeta(f, a, r)); model.Status(e) != 403 {
		t.Fatal("owner policy changed", e)
	}
	task.AcceptancePolicy = policy
	f.Must(a, "request.convert", id, map[string]any{"task": task}, intakeMeta(f, a, r))
	r = intake(t, f, a, id)
	converted := f.Task(r.TaskID)
	if converted.AcceptanceConstraints.ModeSource != "owner" || converted.AcceptanceConstraints.Requirements[0].Source != "owner" {
		t.Fatal(converted.AcceptanceConstraints)
	}
	m := f.Meta(a)
	m.ExpectedVersion = converted.Version
	m.IntegratorEpoch = converted.IntegratorEpoch
	if _, e := f.Apply(a, "collaborator.limit", converted.ID, map[string]any{"limit": 1, "reason": "automatic increase"}, m); model.Status(e) != 403 {
		t.Fatal("owner capacity changed after conversion", e)
	}
	cancelled := createIntake(f, p)
	r = intake(t, f, a, cancelled)
	if _, e := f.Apply(a, "request.cancel", cancelled, map[string]any{"reason": "agent says no"}, intakeMeta(f, a, r)); model.Status(e) != 403 {
		t.Fatal(e)
	}
	f.Must(f.Owner(), "request.cancel", cancelled, map[string]any{"reason": "not needed"}, intakeMeta(f, f.Owner(), r))
	r = intake(t, f, a, cancelled)
	if _, e := f.Apply(a, "request.claim", cancelled, map[string]any{}, intakeMeta(f, a, r)); model.Status(e) != 409 {
		t.Fatal("cancelled request claimed", e)
	}
}
