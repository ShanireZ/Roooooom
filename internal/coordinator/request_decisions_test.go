package coordinator_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/testkit"
)

func intakeQuestion(id model.ID) map[string]any {
	p := requestPayload("", "")
	delete(p, "taskId")
	p["requestId"] = id
	p["blockedAssignmentIds"] = []model.ID{}
	p["blockedActions"] = []string{"request-convert"}
	return p
}
func askedIntake(t *testing.T) (*testkit.Fixture, model.Actor, model.ID, model.ID, intakeView, model.Receipt) {
	t.Helper()
	f := testkit.New(t)
	p, repo := f.Project()
	a := f.Agent(p, "preparer")
	id := createIntake(f, p)
	r := intake(t, f, a, id)
	f.Must(a, "request.claim", id, map[string]any{}, intakeMeta(f, a, r))
	r = intake(t, f, a, id)
	q := f.Must(a, "decision.request", "", intakeQuestion(id), intakeMeta(f, a, r))
	r = intake(t, f, a, id)
	return f, a, p, repo, r, q
}

// A fabricated empty Task or a nonblocking intake question would let ambiguous work start.
func TestRequestDecisionBeforeTask(t *testing.T) {
	f, a, p, _, r, q := askedIntake(t)
	if f.Count("tasks") != 0 {
		t.Fatal("placeholder task created")
	}
	duplicate := f.Must(a, "decision.request", "", intakeQuestion(r.ID), intakeMeta(f, a, r))
	if duplicate.ResourceID != q.ResourceID {
		t.Fatal("same intake topic duplicated")
	}
	payload := map[string]any{"task": testkit.TaskPayload(p, 0)}
	if _, e := f.Apply(a, "request.convert", r.ID, payload, intakeMeta(f, a, r)); model.Status(e) != 409 {
		t.Fatal("pending question ignored", e)
	}
	m := f.Meta(a)
	m.ExpectedVersion = 1
	if _, e := f.Apply(a, "decision.resolve", q.ResourceID, map[string]any{"answer": "a", "optionId": "a"}, m); model.Status(e) != 403 {
		t.Fatal("preparer answered for owner", e)
	}
	m = f.Meta(f.Owner())
	m.ExpectedVersion = 1
	f.Must(f.Owner(), "decision.resolve", q.ResourceID, map[string]any{"answer": "先釐清範圍", "outcome": "revise"}, m)
	r = intake(t, f, a, r.ID)
	cm := intakeMeta(f, a, r)
	cm.DecisionVersions[q.ResourceID] = 1
	if _, e := f.Apply(a, "request.convert", r.ID, payload, cm); model.Status(e) != 409 {
		t.Fatal("revise outcome ignored", e)
	}
	view, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "decision", TargetID: q.ResourceID})
	if e != nil {
		t.Fatal(e)
	}
	var record struct {
		Request struct {
			Version           int64
			CurrentDecisionID model.ID
		}
	}
	if e = json.Unmarshal(view.Data, &record); e != nil {
		t.Fatal(e)
	}
	m = f.Meta(f.Owner())
	m.ExpectedVersion = record.Request.Version
	f.Must(f.Owner(), "decision.supersede", q.ResourceID, map[string]any{"answer": "維持原範圍", "optionId": "a", "supersedesDecisionId": record.Request.CurrentDecisionID}, m)
	r = intake(t, f, a, r.ID)
	if _, e = f.Apply(a, "request.convert", r.ID, payload, intakeMeta(f, a, r)); model.Status(e) != 409 {
		t.Fatal("decision version not acknowledged", e)
	}
	cm = intakeMeta(f, a, r)
	cm.DecisionVersions[q.ResourceID] = 2
	f.Must(a, "request.convert", r.ID, payload, cm)
	r = intake(t, f, a, r.ID)
	task := f.Task(r.TaskID)
	if task.DecisionVersions[q.ResourceID] != 2 {
		t.Fatal("intake decision not inherited", task.DecisionVersions)
	}
	decisions, e := f.Core.Read(t.Context(), a, model.Query{Kind: "decisions", TargetID: task.ID})
	if e != nil {
		t.Fatal(e)
	}
	var questions []struct {
		ID        model.ID
		RequestID model.ID
		TaskID    model.ID
	}
	if e = json.Unmarshal(decisions.Data, &questions); e != nil || len(questions) != 1 || questions[0].RequestID != r.ID || questions[0].TaskID != "" {
		t.Fatal("subject/history rewritten", questions, e)
	}
}

// Failing to route an intake supersede through its conversion link would leave an accepted old artifact valid.
func TestRequestDecisionSupersedeAfterConversion(t *testing.T) {
	f, a, p, repo, r, q := askedIntake(t)
	m := f.Meta(f.Owner())
	m.ExpectedVersion = 1
	resolved := f.Must(f.Owner(), "decision.resolve", q.ResourceID, map[string]any{"answer": "a", "optionId": "a"}, m)
	var response struct{ Decision struct{ ID model.ID } }
	if e := json.Unmarshal(resolved.Result, &response); e != nil {
		t.Fatal(e)
	}
	r = intake(t, f, a, r.ID)
	m = intakeMeta(f, a, r)
	m.DecisionVersions[q.ResourceID] = 1
	f.Must(a, "request.convert", r.ID, map[string]any{"task": testkit.TaskPayload(p, 0)}, m)
	r = intake(t, f, a, r.ID)
	task := f.Task(r.TaskID)
	meta := func() model.Mutation {
		m := f.Meta(a)
		current := f.Task(task.ID)
		m.ExpectedVersion = current.Version
		m.IntegratorEpoch = current.IntegratorEpoch
		m.DecisionVersions[q.ResourceID] = 1
		return m
	}
	assignment := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task.ID, repo), meta())
	am := f.Meta(a)
	am.ExpectedVersion = 1
	f.Must(a, "assignment.claim", assignment.ResourceID, map[string]any{}, am)
	am = f.Meta(a)
	am.ExpectedVersion = 2
	am.ClaimEpoch = 1
	am.DecisionVersions[q.ResourceID] = 1
	delivery := f.Must(a, "delivery.submit", assignment.ResourceID, map[string]any{"repoId": repo, "baseSha": strings.Repeat("a", 40), "headSha": strings.Repeat("b", 40), "branchRef": "intake", "workspaceRef": "synthetic workspace", "summary": "bounded change", "evidenceRefs": []string{"synthetic://checks"}, "unknowns": []string{}, "artifactRefs": []any{}}, am)
	review := meta()
	review.ExpectedVersion = f.Assignment(assignment.ResourceID).Version
	f.Must(a, "delivery.review", assignment.ResourceID, map[string]any{"deliveryRevisionId": delivery.ResourceID, "disposition": "accepted", "reason": "verified"}, review)
	integrated := f.Must(a, "integration.record", task.ID, map[string]any{"deliveryRevisionIds": []model.ID{delivery.ResourceID}, "heads": []any{map[string]any{"repoId": repo, "baseSha": strings.Repeat("a", 40), "headSha": strings.Repeat("b", 40)}}, "checks": []any{map[string]any{"condition": "tests on exact head", "outcome": "passed", "headShas": map[model.ID]string{repo: strings.Repeat("b", 40)}, "evidenceRefs": []string{"synthetic://integration"}}}, "summary": "verified integration", "result": "passed", "excludedAssignments": []any{}, "untested": []string{}}, meta())
	candidate := f.Must(a, "acceptance.candidate", task.ID, map[string]any{"integrationId": integrated.ResourceID}, meta())
	f.Must(a, "acceptance.accept", task.ID, map[string]any{"candidateId": candidate.ResourceID}, meta())
	f.Must(a, "task.complete", task.ID, map[string]any{"candidateId": candidate.ResourceID}, meta())
	before := f.Task(task.ID)
	m = f.Meta(f.Owner())
	m.ExpectedVersion = resolved.Version
	f.Must(f.Owner(), "decision.supersede", q.ResourceID, map[string]any{"answer": "要求重新調整", "outcome": "revise", "supersedesDecisionId": response.Decision.ID}, m)
	after := f.Task(task.ID)
	if after.Phase != "review" || after.CurrentCandidateID != "" || after.LastCompletionID != before.LastCompletionID || after.DecisionVersions[q.ResourceID] != 2 {
		t.Fatal("supersede did not invalidate linked task", after)
	}
	if _, e := f.Apply(a, "acceptance.accept", task.ID, map[string]any{"candidateId": candidate.ResourceID}, meta()); model.Status(e) != 409 {
		t.Fatal("old intake candidate accepted", e)
	}
}

// Intake notifications must reference the right project without a fake Task, and cancellation must suppress delivery.
func TestRequestDecisionCancellationAndNotifications(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "cancelled"}[cancel], func(t *testing.T) {
			f := testkit.New(t)
			p, _ := f.Project()
			other, _ := f.Project()
			a := f.Agent(p, "preparer")
			foreign := f.Agent(other, "foreign")
			if e := privatefs.WriteAtomic(filepath.Dir(f.Store.Path), "credentials/intake-bot.json", []byte(`{"kind":"dingtalk","token":"synthetic-intake","secret":"SECtest"}`)); e != nil {
				t.Fatal(e)
			}
			binding := outbound.Binding{ProjectID: p, Kind: "dingtalk", CredentialRef: "intake-bot", Enabled: true, Fields: []string{"goal", "summary", "projectTitle"}, Events: []string{"decision-pending"}}
			b, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(binding), Meta: f.Meta(f.Owner())})
			if e != nil {
				t.Fatal(e)
			}
			id := createIntake(f, p)
			r := intake(t, f, a, id)
			f.Must(a, "request.claim", id, map[string]any{}, intakeMeta(f, a, r))
			r = intake(t, f, a, id)
			q := f.Must(a, "decision.request", "", intakeQuestion(id), intakeMeta(f, a, r))
			r = intake(t, f, a, id)
			if _, e = f.Core.Read(t.Context(), foreign, model.Query{Kind: "decision", TargetID: q.ResourceID}); model.Status(e) != 403 {
				t.Fatal("foreign question read", e)
			}
			var jobs, bad int
			if e = f.Store.QueryRow(context.Background(), "SELECT count(*) FROM external_jobs WHERE binding_id=? AND request_id=? AND task_id IS NULL", b.ResourceID, id).Scan(&jobs); e != nil || jobs != 1 {
				t.Fatal("no intake notification", jobs, e)
			}
			if e = f.Store.QueryRow(t.Context(), "SELECT count(*) FROM external_jobs WHERE binding_id<>?", b.ResourceID).Scan(&bad); e != nil || bad != 0 || f.Count("tasks") != 0 {
				t.Fatal("wrong target or fake task", bad, e)
			}
			if cancel {
				f.Must(f.Owner(), "request.cancel", id, map[string]any{"reason": "not needed"}, intakeMeta(f, f.Owner(), r))
				view, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "decision", TargetID: q.ResourceID})
				if e != nil {
					t.Fatal(e)
				}
				var d struct{ Request struct{ State string } }
				json.Unmarshal(view.Data, &d)
				if d.Request.State != "withdrawn" {
					t.Fatal("cancel left question pending")
				}
			}
			recorder := &notificationRecorder{}
			queue := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"dingtalk": recorder})
			if e = queue.RunOnce(t.Context(), time.Now()); e != nil {
				t.Fatal(e)
			}
			expected := 1
			if cancel {
				expected = 0
			}
			if recorder.calls != expected {
				t.Fatal("notification delivery after cancel or lost pending", recorder.calls)
			}
		})
	}
}

func TestRequestDecisionPreparationAndFollowUpFence(t *testing.T) {
	f, a, p, _, r, q := askedIntake(t)
	other := f.Agent(p, "other")
	stale := intakeMeta(f, a, r)
	stale.ClaimEpoch--
	if _, e := f.Apply(a, "decision.request", "", intakeQuestion(r.ID), stale); model.Status(e) != 409 {
		t.Fatal("old preparation epoch asked a question", e)
	}
	m := f.Meta(f.Owner())
	m.ExpectedVersion = 1
	resolved := f.Must(f.Owner(), "decision.resolve", q.ResourceID, map[string]any{"answer": "a", "optionId": "a", "authorityFollowUpRequired": true}, m)
	r = intake(t, f, a, r.ID)
	cm := intakeMeta(f, a, r)
	cm.DecisionVersions[q.ResourceID] = 1
	if _, e := f.Apply(a, "request.convert", r.ID, map[string]any{"task": testkit.TaskPayload(p, 0)}, cm); model.Status(e) != 409 {
		t.Fatal("missing follow-up evidence did not block", e)
	}
	fm := f.Meta(other)
	fm.ExpectedVersion = resolved.Version
	fm.ClaimEpoch = r.PreparationEpoch
	proof := map[string]any{"reason": "authority updated", "evidenceRefs": []string{"synthetic://authority-commit"}}
	if _, e := f.Apply(other, "decision.followup", q.ResourceID, proof, fm); model.Status(e) != 409 {
		t.Fatal("other preparer supplied follow-up", e)
	}
	fm = f.Meta(a)
	fm.ExpectedVersion = resolved.Version
	fm.ClaimEpoch = r.PreparationEpoch
	f.Must(a, "decision.followup", q.ResourceID, proof, fm)
	r = intake(t, f, a, r.ID)
	cm = intakeMeta(f, a, r)
	cm.DecisionVersions[q.ResourceID] = 1
	f.Must(a, "request.convert", r.ID, map[string]any{"task": testkit.TaskPayload(p, 0)}, cm)
}
