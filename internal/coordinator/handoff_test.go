package coordinator_test

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestHandoffAndClaimRace(t *testing.T) {
	f, a, b, task, assignment, repo := responsibilityFixture(t, 1)
	project := f.Task(task).ProjectID
	c := f.Agent(project, "competitor")
	m := f.Meta(a)
	m.IntegratorEpoch = 1
	y := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task, repo), m)
	offer := propose(t, f, a, b, task, handoffPayload(a, b, assignment, "continue-worker"))
	handoffMeta := f.Meta(b)
	handoffMeta.ExpectedVersion = f.Task(task).Version
	handoffMeta.IntegratorEpoch = 1
	claimMeta := f.Meta(c)
	claimMeta.ExpectedVersion = 1
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; _, e := f.Apply(b, "integrator.handoff", task, offer, handoffMeta); results <- e }()
	go func() {
		<-start
		_, e := f.Apply(c, "assignment.claim", y.ResourceID, struct{}{}, claimMeta)
		results <- e
	}()
	close(start)
	winners := 0
	for range 2 {
		e := <-results
		if e == nil {
			winners++
		} else if model.Status(e) != 409 {
			t.Fatal(e)
		}
	}
	if winners != 1 || f.Task(task).CollaboratorCount > 1 {
		t.Fatal("handoff/claim race violated final capacity")
	}
}
func TestHandoffRetainsOwnerRequirements(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "a"), f.Agent(p, "b")
	payload := testkit.TaskPayload(p, 0)
	payload.AcceptancePolicy.Mode = "human"
	payload.AcceptancePolicy.OwnerProtected = true
	payload.AcceptancePolicy.Conditions = []string{"owner checks exact head"}
	task := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	m := f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "integrator.claim", task.ResourceID, struct{}{}, m)
	offer := propose(t, f, a, b, task.ResourceID, map[string]any{"fromSessionId": a.SessionID, "toSessionId": b.SessionID, "dispositions": []any{}, "reason": "explicit transfer"})
	accept(t, f, b, task.ResourceID, offer)
	after := f.Task(task.ResourceID)
	if !after.AcceptancePolicy.OwnerProtected || after.AcceptancePolicy.Mode != "human" || len(after.AcceptancePolicy.Conditions) != 1 || after.AcceptancePolicy.Conditions[0] != "owner checks exact head" || after.AcceptancePolicyVersion != 1 {
		t.Fatal("owner requirements lost")
	}
}

func responsibilityFixture(t *testing.T, n int) (*testkit.Fixture, model.Actor, model.Actor, model.ID, model.ID, model.ID) {
	f := testkit.New(t)
	p, repo := f.Project()
	a, b := f.Agent(p, "integrator"), f.Agent(p, "next")
	task := f.Must(a, "task.create", "", testkit.TaskPayload(p, n), f.Meta(a))
	m := f.Meta(a)
	m.IntegratorEpoch = 1
	x := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task.ResourceID, repo), m)
	m = f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "assignment.claim", x.ResourceID, struct{}{}, m)
	return f, a, b, task.ResourceID, x.ResourceID, repo
}
func TestResumeEndedVsUnknown(t *testing.T) {
	f, a, _, task, assignment, _ := responsibilityFixture(t, 0)
	next := f.NewSession(a)
	payload := map[string]any{"expectedSourceSessionId": a.SessionID, "sourceVersion": 1, "responsibilityIds": []model.ID{task, assignment}}
	if _, e := f.Apply(next, "session.resume", next.SessionID, payload, f.Meta(next)); model.Status(e) != 409 {
		t.Fatal("unknown source silently taken", e)
	}
	m := f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "session.end", a.SessionID, map[string]any{"summary": "ended explicitly"}, m)
	payload["sourceVersion"] = 2
	f.Must(next, "session.resume", next.SessionID, payload, f.Meta(next))
	if f.Task(task).IntegratorSessionID != next.SessionID || f.Task(task).IntegratorEpoch != 2 || f.Assignment(assignment).ClaimSessionID != next.SessionID || f.Assignment(assignment).ClaimEpoch != 2 {
		t.Fatal("responsibility did not rebind")
	}
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(a, "assignment.update", assignment, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("ended session wrote", e)
	}
}
func TestResumeCapabilityIsBoundAndSingleUse(t *testing.T) {
	f, a, _, task, assignment, _ := responsibilityFixture(t, 0)
	next := f.NewSession(a)
	m := f.Meta(a)
	m.ExpectedVersion = 1
	checkpoint := f.Must(a, "session.checkpoint", a.SessionID, map[string]any{"summary": "explicit handoff", "handoffTargetSessionId": next.SessionID, "responsibilityIds": []model.ID{assignment}}, m)
	var ref struct {
		Version       int64  `json:"version"`
		CapabilityRef string `json:"handoffCapabilityRef"`
	}
	if e := json.Unmarshal(checkpoint.Result, &ref); e != nil {
		t.Fatal(e)
	}
	payload := map[string]any{"expectedSourceSessionId": a.SessionID, "sourceVersion": ref.Version, "responsibilityIds": []model.ID{assignment, task}, "handoffCapabilityRef": ref.CapabilityRef}
	if _, e := f.Apply(next, "session.resume", next.SessionID, payload, f.Meta(next)); model.Status(e) != 409 {
		t.Fatal("capability expanded", e)
	}
	payload["responsibilityIds"] = []model.ID{assignment}
	f.Must(next, "session.resume", next.SessionID, payload, f.Meta(next))
	if f.Task(task).IntegratorSessionID != a.SessionID {
		t.Fatal("unselected responsibility taken")
	}
	payload["sourceVersion"] = ref.Version + 1
	if _, e := f.Apply(next, "session.resume", next.SessionID, payload, f.Meta(next)); model.Status(e) != 409 {
		t.Fatal("capability reused", e)
	}
}
func handoffPayload(a, b model.Actor, assignment model.ID, disposition string) map[string]any {
	return map[string]any{"fromSessionId": a.SessionID, "toSessionId": b.SessionID, "dispositions": []any{map[string]any{"assignmentId": assignment, "action": disposition}}, "reason": "explicit transfer"}
}
func propose(t *testing.T, f *testkit.Fixture, a, b model.Actor, task model.ID, payload map[string]any) map[string]any {
	t.Helper()
	m := f.Meta(a)
	m.ExpectedVersion = f.Task(task).Version
	m.IntegratorEpoch = f.Task(task).IntegratorEpoch
	r := f.Must(a, "integrator.handoff", task, payload, m)
	var offer map[string]any
	if e := json.Unmarshal(r.Result, &offer); e != nil {
		t.Fatal(e)
	}
	payload["handoffId"] = offer["handoffId"]
	payload["acceptedHandoffVersion"] = offer["version"]
	return payload
}
func accept(t *testing.T, f *testkit.Fixture, b model.Actor, task model.ID, payload map[string]any) {
	t.Helper()
	m := f.Meta(b)
	m.ExpectedVersion = f.Task(task).Version
	m.IntegratorEpoch = f.Task(task).IntegratorEpoch
	f.Must(b, "integrator.handoff", task, payload, m)
}
func TestHandoffAllOrNothing(t *testing.T) {
	f, a, b, task, assignment, _ := responsibilityFixture(t, 0)
	before := f.Task(task)
	m := f.Meta(a)
	m.ExpectedVersion = before.Version
	m.IntegratorEpoch = 1
	payload := handoffPayload(a, b, assignment, "release")
	payload["dispositions"] = []any{}
	if _, e := f.Apply(a, "integrator.handoff", task, payload, m); model.Status(e) != 409 {
		t.Fatal("omitted claim", e)
	}
	if f.Task(task).Version != before.Version || f.Assignment(assignment).ClaimEpoch != 1 {
		t.Fatal("partial rejected handoff")
	}
	payload = handoffPayload(a, b, assignment, "continue-worker")
	if _, e := f.Apply(a, "integrator.handoff", task, payload, m); model.Status(e) != 409 {
		t.Fatal("N=0 hidden worker", e)
	}
	payload = propose(t, f, a, b, task, handoffPayload(a, b, assignment, "release"))
	// Only the target session can accept the exact offer.
	m = f.Meta(a)
	m.ExpectedVersion = before.Version
	m.IntegratorEpoch = 1
	if _, e := f.Apply(a, "integrator.handoff", task, payload, m); model.Status(e) != 403 {
		t.Fatal("sender forged acceptance", e)
	}
	accept(t, f, b, task, payload)
	if f.Task(task).IntegratorID != b.ParticipantID || f.Task(task).IntegratorEpoch != 2 || f.Assignment(assignment).ClaimParticipantID != "" {
		t.Fatal("handoff did not commit")
	}
}
func TestHandoffFinalMembershipCount(t *testing.T) {
	f, a, b, task, assignment, repo := responsibilityFixture(t, 1)
	m := f.Meta(a)
	m.IntegratorEpoch = 1
	workerAssignment := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task, repo), m)
	m = f.Meta(b)
	m.ExpectedVersion = 1
	f.Must(b, "assignment.claim", workerAssignment.ResourceID, struct{}{}, m)
	payload := propose(t, f, a, b, task, handoffPayload(a, b, assignment, "continue-worker"))
	accept(t, f, b, task, payload)
	if f.Task(task).CollaboratorCount != 1 || f.Task(task).IntegratorID != b.ParticipantID || f.Assignment(assignment).ClaimParticipantID != a.ParticipantID {
		t.Fatal("final-state capacity wrong")
	}
}
func TestCommittedRetryAfterHandoff(t *testing.T) {
	f, a, b, task, assignment, _ := responsibilityFixture(t, 0)
	m := f.Meta(a)
	m.ExpectedVersion = f.Task(task).Version
	m.IntegratorEpoch = 1
	payload := map[string]any{"limit": 0, "reason": "explicit setting"}
	r := f.Must(a, "collaborator.limit", task, payload, m)
	offer := propose(t, f, a, b, task, handoffPayload(a, b, assignment, "release"))
	accept(t, f, b, task, offer)
	count := f.Count("events")
	again, e := f.Apply(a, "collaborator.limit", task, payload, m)
	if e != nil || again.EventID != r.EventID || f.Count("events") != count {
		t.Fatal("historical receipt lost", e)
	}
	m.ClientRequestID = "new-request"
	m.ExpectedVersion = f.Task(task).Version
	if _, e = f.Apply(a, "collaborator.limit", task, payload, m); model.Status(e) != 409 {
		t.Fatal("new stale epoch accepted", e)
	}
	if e = f.Identities.Revoke(t.Context(), f.Owner(), a.ParticipantID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.Apply(a, "collaborator.limit", task, payload, m); model.Status(e) != 403 {
		t.Fatal("revoked receipt leaked", e)
	}
}
