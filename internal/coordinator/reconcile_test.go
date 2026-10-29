package coordinator_test

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
	"time"
)

func offlinePayload(f *testkit.Fixture, a model.Actor, project model.ID, kind string, target model.ID, payload any) map[string]any {
	original := model.Command{Kind: kind, TargetID: target, Meta: f.Meta(a), Payload: model.JSON(payload)}
	return map[string]any{"projectId": project, "localId": string((model.RandomIDs{}).NewID()), "originalRequestId": original.Meta.ClientRequestID, "originalCommand": original, "baseCursor": "", "localOccurredAt": time.Now().Add(-time.Hour)}
}
func TestOfflineFactsAreLateAndControlRequiresCurrentReview(t *testing.T) {
	f, a, b, task, x, repo := responsibilityFixture(t, 0)
	project := f.Task(task).ProjectID
	fact := f.Must(a, "offline.reconcile", "", offlinePayload(f, a, project, "message.send", "", messagePayload(project, task, "participants", []model.ID{b.ParticipantID}, "offline work fact")), f.Meta(a))
	var result struct {
		State       string
		CandidateID model.ID
	}
	if e := json.Unmarshal(fact.Result, &result); e != nil || result.State != "recorded-fact" {
		t.Fatal("append-only fact was not recorded as late", e, result)
	}
	var raw string
	if e := f.Store.QueryRow(t.Context(), "SELECT body FROM messages WHERE task_id=?", task).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var message struct {
		Imported        bool
		LocalOccurredAt time.Time
		ReceivedAt      time.Time
	}
	if e := json.Unmarshal([]byte(raw), &message); e != nil || !message.Imported || !message.ReceivedAt.After(message.LocalOccurredAt) {
		t.Fatal("late fact lost provenance", e)
	}
	draft := f.Must(a, "offline.reconcile", "", offlinePayload(f, a, project, "task.complete", task, map[string]any{"candidateId": "offline-candidate"}), f.Meta(a))
	if e := json.Unmarshal(draft.Result, &result); e != nil || result.State != "candidate" {
		t.Fatal(e)
	}
	m := f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	if _, e := f.Apply(a, "assignment.update", x, map[string]any{"state": "active"}, m); model.Status(e) != 409 {
		t.Fatal("affected work resumed before reconciliation review", e)
	}
	other := f.Must(a, "task.create", "", testkit.TaskPayload(project, 0), f.Meta(a))
	m = f.Meta(a)
	m.IntegratorEpoch = 1
	y := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(other.ResourceID, repo), m)
	m = f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "assignment.claim", y.ResourceID, struct{}{}, m)
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	f.Must(a, "assignment.update", y.ResourceID, map[string]any{"state": "active"}, m)
	m = f.Meta(a)
	m.ExpectedVersion = 1
	m.IntegratorEpoch = 1
	f.Must(a, "offline.resolve", result.CandidateID, map[string]any{"disposition": "reissue", "reason": "current integrator reviewed old completion attempt", "evidenceRefs": []string{"synthetic://current-state"}}, m)
	m = f.Meta(a)
	m.ExpectedVersion = 2
	m.ClaimEpoch = 1
	f.Must(a, "assignment.update", x, map[string]any{"state": "active"}, m)
	if f.Task(task).Phase == "done" {
		t.Fatal("candidate review executed old completion")
	}
}
