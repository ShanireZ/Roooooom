package coordinator_test

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
	"time"
)

func requestPayload(task, assignment model.ID) map[string]any {
	return map[string]any{
		"taskId": task, "scopeKey": "interface-choice", "question": "Which approved interface?", "reason": "two incompatible options", "factRefs": []string{"synthetic://contract"}, "options": []any{map[string]any{"id": "a", "label": "Keep interface", "impact": "compatible"}, map[string]any{"id": "b", "label": "Change interface", "impact": "requires migration"}}, "recommendation": "a", "recommendationReason": "minimal scope", "impact": "one assignment", "blockedAssignmentIds": []model.ID{assignment}, "blockedActions": []string{"implementation", "delivery"}, "canContinue": []string{"other assignment"}, "noDecisionBehavior": "wait in affected assignment",
	}
}
func TestOnlyOwnerResolvesScopedDecision(t *testing.T) {
	f, a, _, task, x, repo := responsibilityFixture(t, 0)
	m := f.Meta(a)
	m.IntegratorEpoch = 1
	y := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task, repo), m)
	m = f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "assignment.claim", y.ResourceID, struct{}{}, m)
	request := f.Must(a, "decision.request", "", requestPayload(task, x), f.Meta(a))
	duplicate := f.Must(a, "decision.request", "", requestPayload(task, x), f.Meta(a))
	if duplicate.ResourceID != request.ResourceID {
		t.Fatal("pending topic not deduplicated")
	}
	m = f.Meta(a)
	m.ExpectedVersion = 1
	if _, e := f.Apply(a, "decision.resolve", request.ResourceID, map[string]any{"answer": "a", "optionId": "a"}, m); model.Status(e) != 403 {
		t.Fatal("agent decided for owner", e)
	}
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(a, "assignment.update", x, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("scoped decision did not block", e)
	}
	f.Must(a, "assignment.update", y.ResourceID, map[string]any{"state": "active"}, m)
	m = f.Meta(f.Owner())
	m.ExpectedVersion = 1
	f.Must(f.Owner(), "decision.resolve", request.ResourceID, map[string]any{"answer": "keep interface", "optionId": "a"}, m)
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(a, "assignment.update", x, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("did not acknowledge decision version", e)
	}
	m.DecisionVersions[request.ResourceID] = 1
	f.Must(a, "assignment.update", x, map[string]any{"state": "active"}, m)
}
func TestProtectedAcceptancePolicy(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "integrator")
	payload := testkit.TaskPayload(p, 0)
	payload.AcceptancePolicy.Mode = "human"
	payload.AcceptancePolicy.OwnerProtected = true
	task := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	m := f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "integrator.claim", task.ResourceID, struct{}{}, m)
	m = f.Meta(a)
	m.ExpectedVersion = f.Task(task.ResourceID).Version
	m.IntegratorEpoch = 1
	changed := payload.AcceptancePolicy
	changed.Mode = "integrator"
	changed.OwnerProtected = false
	if _, e := f.Apply(a, "acceptance.policy", task.ResourceID, map[string]any{"policy": changed, "reason": "faster"}, m); model.Status(e) != 403 {
		t.Fatal("owner rule downgraded", e)
	}
	changed = payload.AcceptancePolicy
	changed.Conditions = append(append([]string{}, changed.Conditions...), "extra agent check")
	f.Must(a, "acceptance.policy", task.ResourceID, map[string]any{"policy": changed, "reason": "additional check"}, m)
	after := f.Task(task.ResourceID)
	if after.AcceptancePolicyVersion != 2 || !after.AcceptancePolicy.OwnerProtected || after.AcceptancePolicy.Mode != "human" {
		t.Fatal(after)
	}
	for _, requirement := range after.AcceptanceConstraints.Requirements {
		if requirement.Text == payload.AcceptancePolicy.Conditions[0] && requirement.Source != "owner" {
			t.Fatal("owner source lost")
		}
	}
}

type candidateFixture struct {
	F                                                                    *testkit.Fixture
	Agent                                                                model.Actor
	Acceptor                                                             model.Actor
	TaskID, AssignmentID, DeliveryID, IntegrationID, CandidateID, RepoID model.ID
	SubmitMeta                                                           model.Mutation
	SubmitPayload                                                        map[string]any
}

func makeCandidate(t *testing.T, mode string) candidateFixture {
	t.Helper()
	f, a, b, task, assignment, repo := responsibilityFixture(t, 0)
	if mode != "integrator" {
		m := f.Meta(a)
		m.ExpectedVersion = f.Task(task).Version
		m.IntegratorEpoch = 1
		policy := f.Task(task).AcceptancePolicy
		policy.Mode = mode
		if mode == "designated-agent" {
			policy.AcceptorID = b.ParticipantID
		}
		f.Must(a, "acceptance.policy", task, map[string]any{"policy": policy, "reason": "explicit acceptor"}, m)
	}
	payload := map[string]any{"repoId": repo, "baseSha": strings.Repeat("a", 40), "headSha": strings.Repeat("b", 40), "branchRef": "synthetic-branch", "workspaceRef": "synthetic workspace", "summary": "bounded change", "evidenceRefs": []string{"synthetic://unit-test"}, "unknowns": []string{}, "artifactRefs": []any{}}
	m := f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	delivered := f.Must(a, "delivery.submit", assignment, payload, m)
	if f.Task(task).Phase == "done" {
		t.Fatal("worker submission completed task")
	}
	review := f.Meta(a)
	review.ExpectedVersion = f.Assignment(assignment).Version
	review.IntegratorEpoch = 1
	f.Must(a, "delivery.review", assignment, map[string]any{"deliveryRevisionId": delivered.ResourceID, "disposition": "accepted", "reason": "verified references"}, review)
	version := f.Task(task).AcceptancePolicyVersion
	integration := map[string]any{"deliveryRevisionIds": []model.ID{delivered.ResourceID}, "heads": []any{map[string]any{"repoId": repo, "baseSha": strings.Repeat("a", 40), "headSha": strings.Repeat("b", 40)}}, "checks": []any{map[string]any{"condition": "tests on exact head", "outcome": "passed", "headShas": map[model.ID]string{repo: strings.Repeat("b", 40)}, "evidenceRefs": []string{"synthetic://integration-test"}}}, "summary": "integrator reported checks", "result": "passed", "excludedAssignments": []any{}, "untested": []string{}}
	im := f.Meta(a)
	im.ExpectedVersion = f.Task(task).Version
	im.IntegratorEpoch = 1
	im.AcceptancePolicyVersion = version
	integrated := f.Must(a, "integration.record", task, integration, im)
	cm := f.Meta(a)
	cm.ExpectedVersion = f.Task(task).Version
	cm.IntegratorEpoch = 1
	cm.AcceptancePolicyVersion = version
	candidate := f.Must(a, "acceptance.candidate", task, map[string]any{"integrationId": integrated.ResourceID}, cm)
	return candidateFixture{f, a, b, task, assignment, delivered.ResourceID, integrated.ResourceID, candidate.ResourceID, repo, m, payload}
}
func TestImmutableDeliveryAndStaleCandidate(t *testing.T) {
	c := makeCandidate(t, "integrator")
	changed := map[string]any{}
	for k, v := range c.SubmitPayload {
		changed[k] = v
	}
	changed["headSha"] = strings.Repeat("c", 40)
	if _, e := c.F.Apply(c.Agent, "delivery.submit", c.AssignmentID, changed, c.SubmitMeta); model.Status(e) != 409 {
		t.Fatal("edited revision via replay", e)
	}
	m := c.F.Meta(c.Agent)
	m.ExpectedVersion = c.F.Assignment(c.AssignmentID).Version
	m.ClaimEpoch = 1
	c.F.Must(c.Agent, "delivery.submit", c.AssignmentID, changed, m)
	m = c.F.Meta(c.Agent)
	m.ExpectedVersion = c.F.Task(c.TaskID).Version
	m.IntegratorEpoch = 1
	if _, e := c.F.Apply(c.Agent, "acceptance.accept", c.TaskID, map[string]any{"candidateId": c.CandidateID}, m); model.Status(e) != 409 {
		t.Fatal("stale head accepted", e)
	}
	var body string
	if e := c.F.Store.QueryRow(t.Context(), "SELECT body FROM deliveries WHERE id=?", c.DeliveryID).Scan(&body); e != nil {
		t.Fatal(e)
	}
	var old map[string]any
	if e := json.Unmarshal([]byte(body), &old); e != nil {
		t.Fatal(e)
	}
	if old["headSha"] != strings.Repeat("b", 40) {
		t.Fatal("delivery revision mutated")
	}
}
func TestCompleteRequiresCurrentEvidence(t *testing.T) {
	c := makeCandidate(t, "designated-agent")
	meta := func(a model.Actor) model.Mutation {
		m := c.F.Meta(a)
		m.ExpectedVersion = c.F.Task(c.TaskID).Version
		m.IntegratorEpoch = 1
		m.AcceptancePolicyVersion = c.F.Task(c.TaskID).AcceptancePolicyVersion
		return m
	}
	if _, e := c.F.Apply(c.Agent, "task.complete", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta(c.Agent)); model.Status(e) != 409 {
		t.Fatal("completed without designated acceptance", e)
	}
	if _, e := c.F.Apply(c.Agent, "acceptance.accept", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta(c.Agent)); model.Status(e) != 403 {
		t.Fatal("wrong acceptor", e)
	}
	c.F.Must(c.Acceptor, "acceptance.accept", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta(c.Acceptor))
	c.F.Must(c.Agent, "task.complete", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta(c.Agent))
	if c.F.Task(c.TaskID).Phase != "done" || c.F.Task(c.TaskID).LastCompletionID == "" {
		t.Fatal("no immutable completion snapshot")
	}
}
func TestDecisionSupersedeReopensAndRetainsCompletion(t *testing.T) {
	c := makeCandidate(t, "integrator")
	meta := func() model.Mutation {
		m := c.F.Meta(c.Agent)
		m.ExpectedVersion = c.F.Task(c.TaskID).Version
		m.IntegratorEpoch = 1
		return m
	}
	c.F.Must(c.Agent, "acceptance.accept", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta())
	c.F.Must(c.Agent, "task.complete", c.TaskID, map[string]any{"candidateId": c.CandidateID}, meta())
	before := c.F.Task(c.TaskID)
	question := requestPayload(c.TaskID, c.AssignmentID)
	question["scopeKey"] = "new-scope"
	question["blockedActions"] = []string{"spec-change", "completion"}
	request := c.F.Must(c.Agent, "decision.request", "", question, c.F.Meta(c.Agent))
	if c.F.Task(c.TaskID).Phase != "review" {
		t.Fatal("new blocking question did not reopen done task")
	}
	owner := c.F.Owner()
	m := c.F.Meta(owner)
	m.ExpectedVersion = 1
	c.F.Must(owner, "decision.resolve", request.ResourceID, map[string]any{"answer": "narrow scope", "optionId": "a", "specAmendment": map[string]any{"goal": "Narrowed approved change", "scope": []string{"narrow feature"}, "nonScope": []string{"publication"}, "authorityRefs": []string{"synthetic://new-scope"}, "risk": "normal"}}, m)
	after := c.F.Task(c.TaskID)
	if after.SpecVersion != 2 || after.LastCompletionID != before.LastCompletionID || after.CurrentCandidateID != "" {
		t.Fatal("scope amendment did not fence evidence", after)
	}
	var count int
	if e := c.F.Store.QueryRow(t.Context(), "SELECT count(*) FROM completions WHERE id=?", before.LastCompletionID).Scan(&count); e != nil || count != 1 {
		t.Fatal("completion history erased", e)
	}
}
func TestHighRiskRequiresFormalApproval(t *testing.T) {
	f := testkit.New(t)
	p, repo := f.Project()
	a := f.Agent(p, "integrator")
	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	task := f.Must(a, "task.create", "", payload, f.Meta(a))
	m := f.Meta(a)
	m.IntegratorEpoch = 1
	assignment := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task.ResourceID, repo), m)
	m = f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "assignment.claim", assignment.ResourceID, struct{}{}, m)
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(a, "assignment.update", assignment.ResourceID, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("high risk started without owner decision", e)
	}
	var request model.ID
	if e := f.Store.QueryRow(t.Context(), "SELECT id FROM decision_requests WHERE task_id=? AND state='pending'", task.ResourceID).Scan(&request); e != nil {
		t.Fatal("missing high-risk decision", e)
	}
	owner := f.Owner()
	om := f.Meta(owner)
	om.ExpectedVersion = 1
	f.Must(owner, "decision.resolve", request, map[string]any{"answer": "revise before starting", "optionId": "revise-first"}, om)
	m.DecisionVersions[request] = 1
	if _, e := f.Apply(a, "assignment.update", assignment.ResourceID, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("rejected risk was treated as approval", e)
	}
	var currentDecision model.ID
	if e := f.Store.QueryRow(t.Context(), "SELECT id FROM decisions WHERE request_id=? AND version=1", request).Scan(&currentDecision); e != nil {
		t.Fatal(e)
	}
	om = f.Meta(owner)
	om.ExpectedVersion = 2
	f.Must(owner, "decision.supersede", request, map[string]any{"answer": "authorize bounded implementation only", "optionId": "authorize-bounded", "supersedesDecisionId": currentDecision}, om)
	m.DecisionVersions[request] = 2
	f.Must(a, "assignment.update", assignment.ResourceID, map[string]any{"state": "active"}, m)
}
func TestAuthorityFollowupAndDeadlineAreExplicit(t *testing.T) {
	f, a, _, task, x, _ := responsibilityFixture(t, 0)
	question := requestPayload(task, x)
	question["deadline"] = time.Now().Add(-time.Hour)
	request := f.Must(a, "decision.request", "", question, f.Meta(a))
	view, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "decision", TargetID: request.ResourceID})
	if e != nil {
		t.Fatal(e)
	}
	var state struct{ Request struct{ State string } }
	if e = json.Unmarshal(view.Data, &state); e != nil || state.Request.State != "pending" {
		t.Fatal("deadline implicitly resolved request", e)
	}
	owner := f.Owner()
	m := f.Meta(owner)
	m.ExpectedVersion = 1
	f.Must(owner, "decision.resolve", request.ResourceID, map[string]any{"answer": "use documented interface", "optionId": "a", "authorityFollowUpRequired": true}, m)
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	m.DecisionVersions[request.ResourceID] = 1
	if _, e = f.Apply(a, "assignment.update", x, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("authority follow-up bypassed", e)
	}
	follow := f.Meta(a)
	follow.ExpectedVersion = 2
	follow.IntegratorEpoch = 1
	follow.DecisionVersions[request.ResourceID] = 1
	f.Must(a, "decision.followup", request.ResourceID, map[string]any{"reason": "authority document updated", "evidenceRefs": []string{"synthetic://authority-revision"}}, follow)
	f.Must(a, "assignment.update", x, map[string]any{"state": "active"}, m)
	question["scopeKey"] = "system:risk:1"
	if _, e = f.Apply(a, "decision.request", "", question, f.Meta(a)); model.Status(e) != 403 {
		t.Fatal("agent forged system approval namespace", e)
	}
}
func TestChangedSpecMakesAcceptedDependencyStale(t *testing.T) {
	c := makeCandidate(t, "integrator")
	f := c.F
	m := f.Meta(c.Agent)
	m.IntegratorEpoch = 1
	payload := testkit.AssignmentPayload(c.TaskID, c.RepoID)
	payload.Dependencies = []model.ID{c.AssignmentID}
	dependent := f.Must(c.Agent, "assignment.create", "", payload, m)
	if !f.Assignment(dependent.ResourceID).CanStart {
		t.Fatal("accepted dependency should be ready")
	}
	question := requestPayload(c.TaskID, c.AssignmentID)
	question["blockedActions"] = []string{"spec-change", "completion"}
	request := f.Must(c.Agent, "decision.request", "", question, f.Meta(c.Agent))
	owner := f.Owner()
	m = f.Meta(owner)
	m.ExpectedVersion = 1
	f.Must(owner, "decision.resolve", request.ResourceID, map[string]any{"answer": "scope revision", "optionId": "a", "specAmendment": map[string]any{"goal": "Revised scope", "scope": []string{"revised implementation"}, "nonScope": []string{"external writes"}, "authorityRefs": []string{"synthetic://revision"}, "risk": "normal"}}, m)
	if f.Assignment(dependent.ResourceID).CanStart {
		t.Fatal("stale accepted dependency unlocked work")
	}
}
func TestChangedAssignmentContractInvalidatesDelivery(t *testing.T) {
	c := makeCandidate(t, "integrator")
	m := c.F.Meta(c.Agent)
	m.IntegratorEpoch = 1
	m.ExpectedVersion = c.F.Assignment(c.AssignmentID).Version
	c.F.Must(c.Agent, "assignment.update", c.AssignmentID, map[string]any{"output": "new required behavior", "reason": "contract changed"}, m)
	m = c.F.Meta(c.Agent)
	m.ExpectedVersion = c.F.Task(c.TaskID).Version
	m.IntegratorEpoch = 1
	if _, e := c.F.Apply(c.Agent, "acceptance.accept", c.TaskID, map[string]any{"candidateId": c.CandidateID}, m); model.Status(e) != 409 {
		t.Fatal("accepted old delivery for a changed assignment contract", e)
	}
}
