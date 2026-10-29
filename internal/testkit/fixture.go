package testkit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"testing"
)

type Fixture struct {
	T          testing.TB
	Store      *store.Store
	Core       *coordinator.Service
	Identities *identity.Service
	owner      model.Actor
}

func New(t testing.TB) *Fixture {
	t.Helper()
	ctx := context.Background()
	st, e := store.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	identities := identity.New(st, nil)
	identities.SetOrigin("http://127.0.0.1:7337")
	path, e := identities.IssueOwnerBootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	match := regexp.MustCompile(`name="capability" value="([^"]+)"`).FindSubmatch(b)
	if len(match) != 2 {
		t.Fatal("activation fixture")
	}
	token, _, e := identities.Activate(ctx, string(match[1]))
	if e != nil {
		t.Fatal(e)
	}
	owner, e := identities.Authenticate(ctx, token)
	if e != nil {
		t.Fatal(e)
	}
	return &Fixture{T: t, Store: st, Core: coordinator.New(st, nil, nil), Identities: identities, owner: owner}
}
func (f *Fixture) Owner() model.Actor { return f.owner }
func (f *Fixture) Meta(a model.Actor) model.Mutation {
	i := f.Store.Identity()
	return model.Mutation{ServiceID: i.ServiceID, Generation: i.Generation, ClientRequestID: string((model.RandomIDs{}).NewID()), SessionID: a.SessionID, SpecVersion: 1, AcceptancePolicyVersion: 1, DecisionVersions: map[model.ID]int64{}}
}
func (f *Fixture) Apply(a model.Actor, kind string, target model.ID, payload any, meta model.Mutation) (model.Receipt, error) {
	return f.Core.Apply(context.Background(), a, model.Command{Kind: kind, TargetID: target, Payload: model.JSON(payload), Meta: meta})
}
func (f *Fixture) Must(a model.Actor, kind string, target model.ID, payload any, meta model.Mutation) model.Receipt {
	f.T.Helper()
	r, e := f.Apply(a, kind, target, payload, meta)
	if e != nil {
		f.T.Fatalf("%s: %v", kind, e)
	}
	return r
}
func (f *Fixture) Project() (model.ID, model.ID) {
	r := f.Must(f.Owner(), "project.create", "", coordinator.ProjectPayload{Name: "Synthetic", Repositories: []coordinator.RepositoryPayload{{Name: "example", Reference: "synthetic://example"}}}, f.Meta(f.Owner()))
	var repo model.ID
	if e := f.Store.QueryRow(context.Background(), "SELECT id FROM repositories WHERE project_id=?", r.ResourceID).Scan(&repo); e != nil {
		f.T.Fatal(e)
	}
	return r.ResourceID, repo
}
func (f *Fixture) Agent(project model.ID, name string) model.Actor {
	f.T.Helper()
	ctx := context.Background()
	cap, e := f.Identities.EnableEnrollment(ctx, f.owner, project)
	if e != nil {
		f.T.Fatal(e)
	}
	credentials, e := f.Identities.Enroll(ctx, cap, string((model.RandomIDs{}).NewID()), name)
	if e != nil {
		f.T.Fatal(e)
	}
	a, e := f.Identities.Authenticate(ctx, credentials.Token)
	if e != nil {
		f.T.Fatal(e)
	}
	return a
}
func (f *Fixture) NewSession(a model.Actor) model.Actor {
	f.T.Helper()
	ctx := context.Background()
	credentials, e := f.Identities.NewSession(ctx, a, string((model.RandomIDs{}).NewID()))
	if e != nil {
		f.T.Fatal(e)
	}
	actor, e := f.Identities.Authenticate(ctx, credentials.Token)
	if e != nil {
		f.T.Fatal(e)
	}
	return actor
}
func (f *Fixture) Task(id model.ID) model.Task {
	f.T.Helper()
	view, e := f.Core.Read(context.Background(), f.owner, model.Query{Kind: "task", TargetID: id})
	if e != nil {
		f.T.Fatal(e)
	}
	var task model.Task
	if e = json.Unmarshal(view.Data, &task); e != nil {
		f.T.Fatal(e)
	}
	return task
}
func (f *Fixture) Assignment(id model.ID) model.Assignment {
	f.T.Helper()
	view, e := f.Core.Read(context.Background(), f.owner, model.Query{Kind: "assignment", TargetID: id})
	if e != nil {
		f.T.Fatal(e)
	}
	var a model.Assignment
	if e = json.Unmarshal(view.Data, &a); e != nil {
		f.T.Fatal(e)
	}
	return a
}
func (f *Fixture) Count(table string) int {
	f.T.Helper()
	switch table {
	case "tasks", "assignments", "memberships", "events", "request_receipts":
	default:
		f.T.Fatal("unsupported count")
	}
	var count int
	if e := f.Store.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); e != nil {
		f.T.Fatal(e)
	}
	return count
}
func TaskPayload(project model.ID, n int) model.TaskCreate {
	return model.TaskCreate{ProjectID: project, Goal: "Implement approved synthetic change", Scope: []string{"one feature"}, NonScope: []string{"publication"}, AuthorityRefs: []string{"synthetic owner instruction"}, Risk: "normal", AcceptancePolicy: model.AcceptancePolicy{Mode: "integrator", Conditions: []string{"tests on exact head"}}, CollaboratorLimit: n}
}
func AssignmentPayload(task, repo model.ID) model.AssignmentCreate {
	return model.AssignmentCreate{TaskID: task, Role: "implementer", Input: "approved contract", Output: "commit and evidence", WriteScope: []string{"src/"}, RepoID: repo, BaseRef: "synthetic-base", Dependencies: []model.ID{}}
}
