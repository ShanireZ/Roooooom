package coordinator_test

import (
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func taskMeta(f *testkit.Fixture, a model.Actor, task model.Task) model.Mutation {
	m := f.Meta(a)
	m.ExpectedVersion = task.Version
	m.IntegratorEpoch = task.IntegratorEpoch
	m.SpecVersion = task.SpecVersion
	m.AcceptancePolicyVersion = task.AcceptancePolicyVersion
	m.DecisionVersions = task.DecisionVersions
	return m
}
func TestManualOwnerCapacityCannotBeOverridden(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "integrator")
	r := f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 1), f.Meta(f.Owner()))
	task := f.Task(r.ResourceID)
	f.Must(a, "integrator.claim", task.ID, map[string]any{}, taskMeta(f, a, task))
	task = f.Task(task.ID)
	if _, e := f.Apply(a, "collaborator.limit", task.ID, map[string]any{"limit": 2, "reason": "increase owner requirement"}, taskMeta(f, a, task)); e == nil {
		t.Fatal("agent changed owner capacity")
	}
}
