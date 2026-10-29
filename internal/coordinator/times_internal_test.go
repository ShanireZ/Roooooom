package coordinator

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

// recordingReader keeps every query a read issues.
type recordingReader struct {
	store.Reader
	mu      sync.Mutex
	queries []recordedQuery
}
type recordedQuery struct {
	sql  string
	args []any
}

func (r *recordingReader) record(q string, args []any) {
	r.mu.Lock()
	r.queries = append(r.queries, recordedQuery{q, args})
	r.mu.Unlock()
}
func (r *recordingReader) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	r.record(q, args)
	return r.Reader.QueryContext(ctx, q, args...)
}
func (r *recordingReader) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	r.record(q, args)
	return r.Reader.QueryRowContext(ctx, q, args...)
}

// fullScans counts queries that walk a project's events from the first one.
func (r *recordingReader) fullScans() int {
	n := 0
	for _, q := range r.queries {
		if !strings.Contains(q.sql, "FROM events WHERE scope=?") || strings.Contains(q.sql, "max(seq)") || strings.Contains(q.sql, "LIMIT 1") {
			continue
		}
		if !strings.Contains(q.sql, "seq>?") || (len(q.args) > 1 && q.args[1] == int64(0)) {
			n++
		}
	}
	return n
}

type timesFixture struct {
	ctx     context.Context
	st      *store.Store
	core    *Service
	owner   model.Actor
	project model.ID
	task    model.ID
}

func newTimesFixture(t *testing.T) *timesFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	st, e := store.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	f := &timesFixture{ctx: ctx, st: st, core: New(st, nil, nil), owner: model.Actor{ID: "test-owner", Kind: "human"}}
	project := f.apply(t, "project.create", ProjectPayload{Name: "Synthetic", Repositories: []RepositoryPayload{}})
	f.project = project.ResourceID
	f.task = f.newTask(t, "Synthetic times")
	return f
}

func (f *timesFixture) newTask(t *testing.T, goal string) model.ID {
	t.Helper()
	return f.apply(t, "task.create", map[string]any{"projectId": f.project, "goal": goal, "scope": []string{"fixture"}, "nonScope": []string{}, "authorityRefs": []string{"synthetic://owner"}, "risk": "normal", "acceptancePolicy": map[string]any{"mode": "integrator", "conditions": []string{"tests pass"}}, "collaboratorLimit": 0}).ResourceID
}

func (f *timesFixture) apply(t *testing.T, kind string, payload any) model.Receipt {
	t.Helper()
	i := f.st.Identity()
	r, e := f.core.Apply(f.ctx, f.owner, model.Command{Kind: kind, Meta: model.Mutation{ServiceID: i.ServiceID, Generation: i.Generation, ClientRequestID: string((model.RandomIDs{}).NewID())}, Payload: model.JSON(payload)})
	if e != nil {
		t.Fatalf("%s: %v", kind, e)
	}
	return r
}

func (f *timesFixture) read(t *testing.T, fn func(readTx) error) *recordingReader {
	t.Helper()
	var recorder *recordingReader
	if e := f.st.Read(f.ctx, func(reader store.Reader) error {
		recorder = &recordingReader{Reader: reader}
		return fn(readTx{f.ctx, recorder})
	}); e != nil {
		t.Fatal(e)
	}
	return recorder
}

func TestNeedsYouWithoutBlockersReadsNoTaskTimes(t *testing.T) {
	f := newTimesFixture(t)
	f.newTask(t, "Second table")
	reader := f.read(t, func(tx readTx) error { _, e := f.core.needsYou(tx, ""); return e })
	if n := reader.fullScans(); n != 0 {
		t.Fatalf("needs-you walked the project's events %d times with no blocker in the room", n)
	}
}

func TestTaskTimesFoldOnlyNewEvents(t *testing.T) {
	f := newTimesFixture(t)
	var first map[model.ID]activityTimes
	reader := f.read(t, func(tx readTx) (e error) { first, e = f.core.taskTimes(tx, f.project); return })
	if reader.fullScans() != 1 || first[f.task].Created.IsZero() {
		t.Fatalf("first read: %d full scans, times %+v", reader.fullScans(), first[f.task])
	}
	later := f.newTask(t, "Later table")
	var second map[model.ID]activityTimes
	reader = f.read(t, func(tx readTx) (e error) { second, e = f.core.taskTimes(tx, f.project); return })
	if n := reader.fullScans(); n != 0 {
		t.Fatalf("a read after one new event walked every event again (%d full scans)", n)
	}
	if second[later].Created.IsZero() || second[f.task] != first[f.task] {
		t.Fatalf("folded %+v / %+v after %+v", second[later], second[f.task], first[f.task])
	}
	if _, touched := first[later]; touched {
		t.Fatal("the cached map was written after it was handed out")
	}
	// A cold service computes the same times from scratch.
	var cold map[model.ID]activityTimes
	fresh := New(f.st, nil, nil)
	f.read(t, func(tx readTx) (e error) { cold, e = fresh.taskTimes(tx, f.project); return })
	if !reflect.DeepEqual(cold, second) {
		t.Fatalf("cached %+v; cold %+v", second, cold)
	}
}
