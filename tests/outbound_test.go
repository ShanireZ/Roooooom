package tests

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
	"time"
)

func TestOwnerOnlyOutboundCredentialIsPrivate(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "a")
	token := "synthetic-secret-never-serialize"
	c := model.Command{Kind: "binding.credential", Payload: model.JSON(map[string]any{"kind": "github", "token": token}), Meta: f.Meta(a)}
	if _, e := f.Core.Outbound(context.Background(), a, c); model.Status(e) != 403 {
		t.Fatal(e)
	}
	c.Meta = f.Meta(f.Owner())
	r, e := f.Core.Outbound(context.Background(), f.Owner(), c)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(r.Result), token) {
		t.Fatal("secret in receipt")
	}
	again, e := f.Core.Outbound(context.Background(), f.Owner(), c)
	if e != nil || again.ResourceID != r.ResourceID {
		t.Fatal("ambiguous credential retry", e)
	}
	var body struct {
		CredentialRef string `json:"credentialRef"`
	}
	if e = json.Unmarshal(r.Result, &body); e != nil || body.CredentialRef == "" {
		t.Fatal(e)
	}
	for _, table := range []string{"events", "request_receipts", "bindings", "external_jobs"} {
		_, e = f.Store.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
			rows, e := tx.Query("SELECT * FROM " + table)
			if e != nil {
				return model.Receipt{}, e
			}
			defer rows.Close()
			columns, _ := rows.Columns()
			for rows.Next() {
				values := make([]any, len(columns))
				ptrs := make([]any, len(columns))
				for i := range values {
					ptrs[i] = &values[i]
				}
				if e = rows.Scan(ptrs...); e != nil {
					return model.Receipt{}, e
				}
				if strings.Contains(string(model.JSON(values)), token) {
					t.Fatal("secret persisted in " + table)
				}
			}
			return model.Receipt{}, rows.Err()
		})
		if e != nil {
			t.Fatal(e)
		}
	}
}

func TestOwnerOnlyOutboundConfig(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "a")
	payload := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Private: true, Fields: []string{"taskId", "goal", "state"}, Events: []string{"task.create", "task.complete", "task.reopen", "integration.record"}, CredentialRef: "synthetic", Enabled: true}
	c := model.Command{Kind: "binding.create", Meta: f.Meta(a), Payload: model.JSON(payload)}
	if _, e := f.Core.Outbound(context.Background(), a, c); model.Status(e) != 403 {
		t.Fatal("agent config", e)
	}
	c.Meta = f.Meta(f.Owner())
	r, e := f.Core.Outbound(context.Background(), f.Owner(), c)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Result) == 0 {
		t.Fatal("no config")
	}
	bad := payload
	bad.Repository = "o/r/../../else"
	c.Payload = model.JSON(bad)
	c.Meta = f.Meta(f.Owner())
	if _, e = f.Core.Outbound(context.Background(), f.Owner(), c); model.Status(e) != 400 {
		t.Fatal(e)
	}
}

type fakeAdapter struct {
	calls          int
	fail           bool
	reconcileState string
	store          *store.Store
	t              *testing.T
}

func TestOwnerOnlyOutboundHTTP(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	r, e := f.core.Apply(ctx, owner, model.Command{Kind: "project.create", Meta: model.Mutation{ServiceID: f.store.Identity().ServiceID, Generation: f.store.Identity().Generation, ClientRequestID: "p"}, Payload: model.JSON(map[string]any{"name": "synthetic", "repositories": []any{}})})
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(ctx, owner, r.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	creds, e := f.ids.Enroll(ctx, cap, "a", "a")
	if e != nil {
		t.Fatal(e)
	}
	if w := f.request("POST", "/api/v1/commands/binding.create", "{secret-sentinel", creds.Token, ""); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.request("GET", "/api/v1/queries/bindings", "", creds.Token, ""); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := f.request("POST", "/api/v1/commands/binding.create", "{}", "owner", ""); w.Code != 403 {
		t.Fatal("missing origin", w.Code)
	}
}

func TestGitHubQueueRecoveryAndBindingFence(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	b := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Private: true, Fields: []string{"taskId", "state"}, Events: []string{"task.create"}, CredentialRef: "synthetic", Enabled: true}
	bind, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())})
	if e != nil {
		t.Fatal(e)
	}
	f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	_, e = f.Store.Write(context.Background(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("UPDATE external_jobs SET status='sending'")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	a := &fakeAdapter{store: f.Store, t: t}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"github": a})
	if e = q.Recover(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = q.RunOnce(context.Background(), time.Now()); e != nil {
		t.Fatal(e)
	}
	if a.calls != 0 {
		t.Fatal("recovered sending blindly delivered")
	}
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = 1
	_, e = f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.disable", TargetID: bind.ResourceID, Payload: model.JSON(struct{}{}), Meta: meta})
	if e != nil {
		t.Fatal(e)
	}
	b.Repository = "o/other"
	b.RepositoryID = 99
	_, e = f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())})
	if e != nil {
		t.Fatal(e)
	}
	if e = q.RunOnce(context.Background(), time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	if a.calls != 0 {
		t.Fatal("old job replayed on changed binding")
	}
}

func TestGitHubOwnerRetryCannotBlindlyReplayUncertain(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	binding := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Private: true, Fields: []string{"taskId"}, Events: []string{"task.create"}, CredentialRef: "synthetic", Enabled: true}
	_, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(binding), Meta: f.Meta(f.Owner())})
	if e != nil {
		t.Fatal(e)
	}
	f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	a := &fakeAdapter{store: f.Store, t: t, fail: true}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"github": a})
	if e = q.RunOnce(context.Background(), time.Now()); e != nil {
		t.Fatal(e)
	}
	var id model.ID
	var version int64
	if e = f.Store.QueryRow(context.Background(), "SELECT id,version FROM external_jobs LIMIT 1").Scan(&id, &version); e != nil {
		t.Fatal(e)
	}
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = version
	c := model.Command{Kind: "binding.retry", TargetID: id, Meta: meta, Payload: model.JSON(map[string]any{"mode": "retry", "reason": "owner review"})}
	if _, e = f.Core.Outbound(context.Background(), f.Owner(), c); model.Status(e) != 409 {
		t.Fatal("blind owner retry", e)
	}
	c.Meta = f.Meta(f.Owner())
	c.Meta.ExpectedVersion = version
	c.Payload = model.JSON(map[string]any{"mode": "reconcile", "reason": "inspect existing marker"})
	if _, e = f.Core.Outbound(context.Background(), f.Owner(), c); e != nil {
		t.Fatal(e)
	}
	if e = q.RunOnce(context.Background(), time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	if a.calls != 1 {
		t.Fatal("owner retry created duplicate")
	}
}

func TestGitHubRepositoryGlobalBudget(t *testing.T) {
	f := testkit.New(t)
	for i := 0; i < 2; i++ {
		p, _ := f.Project()
		b := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Fields: []string{"taskId"}, Events: []string{"task.create"}, CredentialRef: "synthetic", Enabled: true}
		if _, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())}); e != nil {
			t.Fatal(e)
		}
		f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	}
	a := &fakeAdapter{store: f.Store, t: t}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"github": a})
	now := time.Now()
	for i := 0; i < 2; i++ {
		if e := q.RunOnce(context.Background(), now); e != nil {
			t.Fatal(e)
		}
	}
	if a.calls != 1 {
		t.Fatal("same repository exceeds mutation budget", a.calls)
	}
}

func TestGitHubMetadataSummaryBounded(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	b := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Fields: []string{"taskId", "goal", "scope", "acceptance"}, Events: []string{"task.create"}, CredentialRef: "synthetic", Enabled: true}
	if _, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())}); e != nil {
		t.Fatal(e)
	}
	payload := testkit.TaskPayload(p, 0)
	payload.Scope = []string{}
	payload.Goal = strings.Repeat("😀", 1000)
	payload.AcceptancePolicy.Conditions = []string{}
	for i := 0; i < 100; i++ {
		payload.Scope = append(payload.Scope, strings.Repeat("😀", 1000)+string(model.JSON(i)))
		payload.AcceptancePolicy.Conditions = append(payload.AcceptancePolicy.Conditions, strings.Repeat("😀", 1000)+string(model.JSON(i)))
	}
	f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	var raw string
	if e := f.Store.QueryRow(context.Background(), "SELECT body FROM external_jobs LIMIT 1").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var job outbound.Job
	json.Unmarshal([]byte(raw), &job)
	if len(job.Text) > 32*1024 {
		t.Fatal("unbounded remote summary", len(job.Text))
	}
}

func (a *fakeAdapter) Deliver(ctx context.Context, j outbound.Job) (outbound.Result, error) {
	a.calls++
	// A write during delivery proves the queue holds no database transaction.
	_, e := a.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": "test.net"}))
		return model.Receipt{}, e
	})
	if e != nil {
		a.t.Fatal(e)
	}
	if a.fail {
		return outbound.Result{State: "uncertain"}, nil
	}
	return outbound.Result{State: "delivered", RemoteID: "8", RemoteURL: "https://github.com/o/r/issues/8"}, nil
}
func (a *fakeAdapter) Reconcile(ctx context.Context, j outbound.Job) (outbound.Result, error) {
	if a.reconcileState != "" {
		return outbound.Result{State: a.reconcileState, ErrorCode: "metadata-unavailable"}, nil
	}
	return outbound.Result{State: "uncertain", ErrorCode: "not-yet-visible"}, nil
}

func TestGitHubUncertaintySurvivesReconcileFailures(t *testing.T) {
	for _, state := range []string{"retry", "paused"} {
		t.Run(state, func(t *testing.T) {
			f := testkit.New(t)
			p, _ := f.Project()
			b := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Fields: []string{"taskId"}, Events: []string{"task.create"}, CredentialRef: "synthetic", Enabled: true}
			if _, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())}); e != nil {
				t.Fatal(e)
			}
			f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
			a := &fakeAdapter{store: f.Store, t: t, fail: true, reconcileState: state}
			q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"github": a})
			start := time.Now()
			for i := 0; i < 2; i++ {
				if e := q.RunOnce(t.Context(), start.Add(time.Duration(i)*2*time.Minute)); e != nil {
					t.Fatal(e)
				}
			}
			if state == "paused" {
				var id model.ID
				var version int64
				f.Store.QueryRow(t.Context(), "SELECT id,version FROM external_jobs LIMIT 1").Scan(&id, &version)
				m := f.Meta(f.Owner())
				m.ExpectedVersion = version
				_, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.retry", TargetID: id, Meta: m, Payload: model.JSON(map[string]any{"mode": "retry", "reason": "credential repaired"})})
				if model.Status(e) != 409 {
					t.Fatal("forgot uncertain effect after reconcile paused", e)
				}
			}
			if e := q.RunOnce(t.Context(), start.Add(5*time.Minute)); e != nil {
				t.Fatal(e)
			}
			if a.calls != 1 {
				t.Fatal("uncertain effect became a blind create", a.calls)
			}
		})
	}
}

func TestOwnerOnlyOutboundCredentialRotationPinsOriginalBinding(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	b := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Fields: []string{"taskId"}, Events: []string{"task.create"}, CredentialRef: "old-reference", Enabled: true}
	binding, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(b), Meta: f.Meta(f.Owner())})
	if e != nil {
		t.Fatal(e)
	}
	f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	a := &fakeAdapter{store: f.Store, t: t, fail: true}
	queue := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"github": a})
	if e = queue.RunOnce(t.Context(), time.Now()); e != nil {
		t.Fatal(e)
	}
	credential, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.credential", Payload: model.JSON(map[string]any{"kind": "github", "token": "synthetic-new-pat"}), Meta: f.Meta(f.Owner())})
	if e != nil {
		t.Fatal(e)
	}
	var secret struct {
		CredentialRef string `json:"credentialRef"`
	}
	json.Unmarshal(credential.Result, &secret)
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = 1
	rotated, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.rotate", TargetID: binding.ResourceID, Payload: model.JSON(map[string]any{"credentialRef": secret.CredentialRef, "reason": "PAT rotation for the same repository"}), Meta: meta})
	if e != nil {
		t.Fatal(e)
	}
	var after outbound.Binding
	json.Unmarshal(rotated.Result, &after)
	if after.Repository != "o/r" || after.RepositoryID != 42 || after.ID != binding.ResourceID || after.CredentialRef != secret.CredentialRef || after.Version != 2 {
		t.Fatal(after)
	}
	if e = queue.RunOnce(t.Context(), time.Now().Add(3*time.Minute)); e != nil {
		t.Fatal(e)
	}
	if a.calls != 1 {
		t.Fatal("rotation replayed uncertain write")
	}
}
func TestRemoteEditsNeverDriveTask(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	binding := outbound.Binding{ProjectID: p, Kind: "github", Repository: "o/r", RepositoryID: 42, Private: true, Fields: []string{"taskId", "goal", "state"}, Events: []string{"task.create", "task.cancel"}, CredentialRef: "synthetic", Enabled: true}
	_, e := f.Core.Outbound(context.Background(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(binding), Meta: f.Meta(f.Owner())})
	if e != nil {
		t.Fatal(e)
	}
	r := f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	task := f.Task(r.ResourceID)
	a := &fakeAdapter{store: f.Store, t: t, fail: true}
	q := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"github": a})
	if e = q.RunOnce(context.Background(), time.Now()); e != nil {
		t.Fatal(e)
	}
	if e = q.RunOnce(context.Background(), time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	if a.calls != 1 {
		t.Fatal("blind duplicate", a.calls)
	}
	if f.Task(task.ID).Version != task.Version {
		t.Fatal("remote changed task")
	}
	var raw string
	if e = f.Store.QueryRow(context.Background(), "SELECT body FROM external_jobs LIMIT 1").Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var job outbound.Job
	if e = json.Unmarshal([]byte(raw), &job); e != nil {
		t.Fatal(e)
	}
	if job.BindingID == "" || job.TaskID != task.ID || job.Marker == "" {
		t.Fatal(job)
	}
}
