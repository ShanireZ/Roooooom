package tests

import (
	"bytes"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"
)

func percentile95(values []time.Duration) time.Duration {
	copy := append([]time.Duration{}, values...)
	sort.Slice(copy, func(i, j int) bool { return copy[i] < copy[j] })
	return copy[(95*len(copy)+99)/100-1]
}
func TestCapacityAndLatency(t *testing.T) {
	f := testkit.New(t)
	project, _ := f.Project()
	agents := []model.Actor{}
	for i := range 20 {
		agents = append(agents, f.Agent(project, "participant-"+strconv.Itoa(i)))
	}
	task := f.Must(agents[0], "task.create", "", testkit.TaskPayload(project, 9), f.Meta(agents[0]))
	metadata := []time.Duration{}
	waits := []time.Duration{}
	for round := 0; round < 20; round++ {
		cursors := []string{}
		for _, actor := range agents[:10] {
			page, e := f.Core.Sync(t.Context(), actor, "")
			if e != nil {
				t.Fatal(e)
			}
			cursors = append(cursors, page.Cursor)
		}
		var wg sync.WaitGroup
		returns := make(chan time.Time, 10)
		failures := make(chan error, 10)
		start := make(chan struct{})
		for i, actor := range agents[:10] {
			wg.Add(1)
			go func(a model.Actor, cursor string) {
				defer wg.Done()
				<-start
				_, e := f.Core.Wait(t.Context(), a, cursor, 2*time.Second)
				returns <- time.Now()
				failures <- e
			}(actor, cursors[i])
		}
		close(start)
		began := time.Now()
		meta := f.Meta(agents[0])
		meta.IntegratorEpoch = 1
		f.Must(agents[0], "message.send", "", map[string]any{"projectId": project, "taskId": task.ResourceID, "threadId": "capacity", "purpose": "status", "priority": "normal", "body": "normal-disk capacity witness", "recipientMode": "project", "recipientIds": []model.ID{}, "actionRequired": false}, meta)
		committed := time.Now()
		metadata = append(metadata, committed.Sub(began))
		wg.Wait()
		close(returns)
		close(failures)
		for e := range failures {
			if e != nil {
				t.Fatal(e)
			}
		}
		for returned := range returns {
			delay := returned.Sub(committed)
			if delay < 0 {
				delay = 0
			}
			waits = append(waits, delay)
		}
	}
	p95metadata, p95wait := percentile95(metadata), percentile95(waits)
	t.Logf("normal disk; OS=%s/%s CPUs=%d; participants=20 simultaneous requesting sessions=10; broadcasts=20; waits=200; metadata_p95=%s post_commit_wait_p95=%s", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), p95metadata, p95wait)
	if p95metadata > 300*time.Millisecond || p95wait > time.Second {
		t.Fatal("approved local capacity budget exceeded")
	}
	// Product statistics must expose bounded counters/latency, never message bodies.
	view, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "metrics"})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(view.Data, []byte("normal-disk capacity witness")) {
		t.Fatal("message body leaked into metrics")
	}
}
