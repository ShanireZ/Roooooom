package coordinator

import (
	"context"
	"path/filepath"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"sync"
	"testing"
	"time"
)

func TestWaitRegistrationRace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st, e := store.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	core := New(st, nil, nil)
	owner := model.Actor{ID: "test-owner", Kind: "human"}
	ids := identity.New(st, nil)
	meta := func(a model.Actor) model.Mutation {
		i := st.Identity()
		return model.Mutation{ServiceID: i.ServiceID, Generation: i.Generation, ClientRequestID: string((model.RandomIDs{}).NewID()), SessionID: a.SessionID}
	}
	project, e := core.Apply(ctx, owner, model.Command{Kind: "project.create", Meta: meta(owner), Payload: model.JSON(ProjectPayload{Name: "Synthetic", Repositories: []RepositoryPayload{}})})
	if e != nil {
		t.Fatal(e)
	}
	capability, e := ids.EnableEnrollment(ctx, owner, project.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := ids.Enroll(ctx, capability, "worker", "worker")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := ids.Authenticate(ctx, credentials.Token)
	if e != nil {
		t.Fatal(e)
	}
	initial, e := core.Sync(ctx, agent, "")
	if e != nil {
		t.Fatal(e)
	}
	registered, proceed := make(chan struct{}), make(chan struct{})
	var once sync.Once
	core.waitRegistered = func() {
		once.Do(func() {
			close(registered)
			select {
			case <-proceed:
			case <-ctx.Done():
			}
		})
	}
	result := make(chan SyncPage, 1)
	failure := make(chan error, 1)
	go func() { page, e := core.Wait(ctx, agent, initial.Cursor, time.Second); result <- page; failure <- e }()
	select {
	case <-registered:
	case <-ctx.Done():
		t.Fatal("wait never registered")
	}
	// Commit precisely between registration and the durable-log recheck.
	_, e = core.Apply(ctx, owner, model.Command{Kind: "message.send", Meta: meta(owner), Payload: model.JSON(messagePayload{ProjectID: project.ResourceID, ThreadID: "race", Purpose: "discussion", Priority: "normal", Body: "durable race witness", RecipientMode: "participants", RecipientIDs: []model.ID{agent.ParticipantID}})})
	if e != nil {
		t.Fatal(e)
	}
	close(proceed)
	select {
	case page := <-result:
		if e := <-failure; e != nil || len(page.Events) != 1 {
			t.Fatal("missed registration-window event", page, e)
		}
	case <-ctx.Done():
		t.Fatal("lost wakeup")
	}
	core.observationMu.Lock()
	waiting := core.observations[agent.SessionID].Waiting
	core.observationMu.Unlock()
	if waiting != 0 {
		t.Fatal("leaked waiter")
	}
	current, e := core.Sync(ctx, agent, "")
	if e != nil {
		t.Fatal(e)
	}
	idle := make(chan struct{})
	idleContext, idleCancel := context.WithCancel(ctx)
	var idleOnce sync.Once
	core.waitRegistered = nil
	core.waitIdle = func() { idleOnce.Do(func() { close(idle); <-idleContext.Done() }) }
	done := make(chan error, 1)
	go func() { _, e := core.Wait(idleContext, agent, current.Cursor, time.Second); done <- e }()
	select {
	case <-idle:
	case <-ctx.Done():
		t.Fatal("wait did not become idle")
	}
	if st.OpenReaders() != 0 {
		t.Fatal("long wait retained a read transaction")
	}
	// A native write can commit while the waiter is parked.
	if _, e = core.Apply(ctx, owner, model.Command{Kind: "project.create", Meta: meta(owner), Payload: model.JSON(ProjectPayload{Name: "While waiting", Repositories: []RepositoryPayload{}})}); e != nil {
		t.Fatal(e)
	}
	idleCancel()
	if e = <-done; e != context.Canceled {
		t.Fatal("wait cancellation was not propagated", e)
	}
	core.observationMu.Lock()
	waiting = core.observations[agent.SessionID].Waiting
	core.observationMu.Unlock()
	if waiting != 0 {
		t.Fatal("cancelled request leaked waiter")
	}
}

type frozenClock struct{ now time.Time }

func (f *frozenClock) Now() time.Time { return f.now }
func TestStalePresenceDoesNotReleaseResponsibilities(t *testing.T) {
	st, e := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	clock := &frozenClock{time.Now().UTC()}
	core := New(st, clock, nil)
	// Presence is an observation, never a claim expiry mechanism.
	_, e = st.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO participants VALUES('p',1,'{}'); INSERT INTO sessions VALUES('s','p',1,'active',?)", string(model.JSON(map[string]any{"lastCheckpointAt": clock.now})))
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	clock.now = clock.now.Add(91 * time.Second)
	e = st.Read(t.Context(), func(reader store.Reader) error {
		view, e := core.sessionView(readTx{t.Context(), reader}, "s")
		if e != nil {
			return e
		}
		if view["presence"] != "unknown" || view["status"] != "active" {
			t.Fatal(view)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
