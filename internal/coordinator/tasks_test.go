package coordinator_test

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
	"sync"
	"testing"
)

func TestTaskContractAndTokenScope(t *testing.T) {
	f := testkit.New(t)
	p, repo := f.Project()
	a := f.Agent(p, "integrator")
	raw := map[string]any{}
	b := model.JSON(testkit.TaskPayload(p, 0))
	if e := json.Unmarshal(b, &raw); e != nil {
		t.Fatal(e)
	}
	delete(raw, "collaboratorLimit")
	if _, e := f.Apply(a, "task.create", "", raw, f.Meta(a)); model.Status(e) != 400 {
		t.Error("missing explicit N accepted", e)
	}
	ready := f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	if _, e := f.Apply(f.Owner(), "assignment.create", "", testkit.AssignmentPayload(ready.ResourceID, repo), f.Meta(f.Owner())); model.Status(e) != 409 {
		t.Error("split before integrator", e)
	}
	// The participant may be granted a second project later; this session token remains scoped to p.
	other, _ := f.Project()
	f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(other, 0), f.Meta(f.Owner()))
	cap, e := f.Identities.EnableEnrollment(context.Background(), f.Owner(), other)
	if e != nil {
		t.Fatal(e)
	}
	_ = cap
	_, e = f.Store.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO participant_projects VALUES(?,?)", a.ParticipantID, other)
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	view, e := f.Core.Read(context.Background(), a, model.Query{Kind: "tasks"})
	if e != nil {
		t.Fatal(e)
	}
	var tasks []model.Task
	if e = json.Unmarshal(view.Data, &tasks); e != nil {
		t.Fatal(e)
	}
	for _, task := range tasks {
		if task.ProjectID != p {
			t.Fatal("session token scope leaked", task.ProjectID)
		}
	}
}

func TestTaskOriginsAndDesignatedIntegrator(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "a")
	b := f.Agent(p, "b")
	first := f.Must(a, "task.create", "", testkit.TaskPayload(p, 1), f.Meta(a))
	task := f.Task(first.ResourceID)
	if task.IntegratorID != a.ParticipantID || task.IntegratorEpoch != 1 || task.Phase != "active" {
		t.Fatal(task)
	}
	payload := testkit.TaskPayload(p, 1)
	payload.DesignatedIntegratorID = b.ParticipantID
	r := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	task = f.Task(r.ResourceID)
	if task.IntegratorID != "" || task.Phase != "ready" {
		t.Fatal(task)
	}
	meta := f.Meta(a)
	meta.ExpectedVersion = task.Version
	if _, e := f.Apply(a, "integrator.claim", task.ID, struct{}{}, meta); model.Status(e) != 403 {
		t.Fatal("designated integrator bypass", e)
	}
	meta = f.Meta(b)
	meta.ExpectedVersion = task.Version
	f.Must(b, "integrator.claim", task.ID, struct{}{}, meta)
	if task = f.Task(task.ID); task.IntegratorID != b.ParticipantID {
		t.Fatal(task)
	}
}

func TestFirstIntegratorClaimIsAtomic(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "a"), f.Agent(p, "b")
	task := f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, actor := range []model.Actor{a, b} {
		go func(actor model.Actor) {
			<-start
			m := f.Meta(actor)
			m.ExpectedVersion = 1
			_, e := f.Apply(actor, "integrator.claim", task.ResourceID, struct{}{}, m)
			results <- e
		}(actor)
	}
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
	if winners != 1 || f.Count("memberships") != 1 || f.Task(task.ResourceID).IntegratorEpoch != 1 {
		t.Fatal("integrator race")
	}
}
func TestParticipantCapacityNeedsExplicitExit(t *testing.T) {
	f := testkit.New(t)
	p, repo := f.Project()
	a, b, c := f.Agent(p, "integrator"), f.Agent(p, "worker"), f.Agent(p, "next")
	task := f.Must(a, "task.create", "", testkit.TaskPayload(p, 1), f.Meta(a))
	create := func() model.ID {
		m := f.Meta(a)
		m.IntegratorEpoch = 1
		return f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task.ResourceID, repo), m).ResourceID
	}
	x, y, z := create(), create(), create()
	for _, id := range []model.ID{x, y} {
		m := f.Meta(b)
		m.ExpectedVersion = 1
		f.Must(b, "assignment.claim", id, struct{}{}, m)
	}
	if f.Task(task.ResourceID).CollaboratorCount != 1 {
		t.Fatal("counted claims instead of participants")
	}
	m := f.Meta(b)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(b, "assignment.release", x, map[string]any{"reason": "exit", "exitTask": true}, m); model.Status(e) != 409 {
		t.Fatal("exit while other claim held", e)
	}
	f.Must(b, "assignment.release", x, map[string]any{"reason": "release"}, m)
	m = f.Meta(c)
	m.ExpectedVersion = 1
	if _, e := f.Apply(c, "assignment.claim", z, struct{}{}, m); model.Status(e) != 409 {
		t.Fatal("implicit slot reclamation", e)
	}
	m = f.Meta(b)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	f.Must(b, "assignment.release", y, map[string]any{"reason": "exit", "exitTask": true}, m)
	m = f.Meta(c)
	m.ExpectedVersion = 1
	f.Must(c, "assignment.claim", z, struct{}{}, m)
	if f.Task(task.ResourceID).CollaboratorCount != 1 {
		t.Fatal("exit did not release slot")
	}
}
func TestConcurrentClaimsAndMembershipLimit(t *testing.T) {
	for _, sameAssignment := range []bool{true, false} {
		t.Run(map[bool]string{true: "same claim", false: "capacity"}[sameAssignment], func(t *testing.T) {
			f := testkit.New(t)
			p, repo := f.Project()
			integrator := f.Agent(p, "integrator")
			a := f.Agent(p, "a")
			b := f.Agent(p, "b")
			task := f.Must(integrator, "task.create", "", testkit.TaskPayload(p, 1), f.Meta(integrator))
			create := func() model.ID {
				m := f.Meta(integrator)
				m.IntegratorEpoch = 1
				return f.Must(integrator, "assignment.create", "", testkit.AssignmentPayload(task.ResourceID, repo), m).ResourceID
			}
			x, y := create(), model.ID("")
			if sameAssignment {
				y = x
			} else {
				y = create()
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for i, a := range []model.Actor{a, b} {
				wg.Add(1)
				target := []model.ID{x, y}[i]
				go func(actor model.Actor, id model.ID) {
					defer wg.Done()
					<-start
					m := f.Meta(actor)
					m.ExpectedVersion = 1
					_, e := f.Apply(actor, "assignment.claim", id, struct{}{}, m)
					results <- e
				}(a, target)
			}
			close(start)
			wg.Wait()
			close(results)
			winners := 0
			for e := range results {
				if e == nil {
					winners++
				} else if model.Status(e) != 409 {
					t.Fatal(e)
				}
			}
			if winners != 1 || f.Task(task.ResourceID).CollaboratorCount != 1 {
				t.Fatal("non-atomic claim", winners)
			}
			m := f.Meta(integrator)
			m.ExpectedVersion = f.Task(task.ResourceID).Version
			m.IntegratorEpoch = 1
			if _, e := f.Apply(integrator, "collaborator.limit", task.ResourceID, map[string]any{"limit": 0, "reason": "shrink"}, m); model.Status(e) != 409 {
				t.Fatal("capacity shrink", e)
			}
		})
	}
}
func TestDependenciesAndZeroCollaborators(t *testing.T) {
	f := testkit.New(t)
	p, repo := f.Project()
	a := f.Agent(p, "integrator")
	worker := f.Agent(p, "worker")
	r := f.Must(a, "task.create", "", testkit.TaskPayload(p, 0), f.Meta(a))
	if !f.Task(r.ResourceID).CanStart {
		t.Fatal("N=0 cannot start")
	}
	create := func(payload model.AssignmentCreate) model.ID {
		m := f.Meta(a)
		m.IntegratorEpoch = 1
		return f.Must(a, "assignment.create", "", payload, m).ResourceID
	}
	x := create(testkit.AssignmentPayload(r.ResourceID, repo))
	payload := testkit.AssignmentPayload(r.ResourceID, repo)
	payload.Dependencies = []model.ID{x}
	y := create(payload)
	m := f.Meta(worker)
	m.ExpectedVersion = 1
	if _, e := f.Apply(worker, "assignment.claim", x, struct{}{}, m); model.Status(e) != 409 {
		t.Fatal("N=0 worker", e)
	}
	m = f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "assignment.claim", y, struct{}{}, m)
	if f.Assignment(y).CanStart {
		t.Fatal("unmet dependency can start")
	}
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(a, "assignment.update", y, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("dependency bypass", e)
	}
	m = f.Meta(a)
	m.ExpectedVersion = 1
	m.IntegratorEpoch = 1
	if _, e := f.Apply(a, "assignment.update", x, map[string]any{"dependencies": []model.ID{y}}, m); model.Status(e) != 400 {
		t.Fatal("cycle accepted", e)
	}
	other, _ := f.Project()
	foreign := f.Agent(other, "foreign")
	if _, e := f.Core.Read(context.Background(), foreign, model.Query{Kind: "task", TargetID: r.ResourceID}); model.Status(e) != 403 {
		t.Fatal("read across project", e)
	}
}
