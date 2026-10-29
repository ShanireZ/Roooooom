package coordinator_test

import (
	"context"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"testing"
	"time"
)

type notificationRecorder struct{ calls int }

func (r *notificationRecorder) Deliver(context.Context, outbound.Job) (outbound.Result, error) {
	r.calls++
	return outbound.Result{State: "accepted"}, nil
}
func (r *notificationRecorder) Reconcile(ctx context.Context, j outbound.Job) (outbound.Result, error) {
	return r.Deliver(ctx, j)
}
func TestHumanAcceptanceNotificationUsesCurrentCandidate(t *testing.T) {
	for _, state := range []string{"pending", "accepted", "policy-changed"} {
		t.Run(state, func(t *testing.T) {
			c := makeCandidate(t, "human")
			f := c.F
			task := f.Task(c.TaskID)
			root := filepath.Dir(f.Store.Path)
			if e := privatefs.WriteAtomic(root, "credentials/bot.json", []byte(`{"kind":"dingtalk","token":"synthetic","secret":"SECtest"}`)); e != nil {
				t.Fatal(e)
			}
			binding := outbound.Binding{ProjectID: task.ProjectID, Kind: "dingtalk", CredentialRef: "bot", Enabled: true, Fields: []string{"taskId", "goal"}, Events: []string{"human-acceptance"}}
			_, e := f.Core.Outbound(t.Context(), f.Owner(), model.Command{Kind: "binding.create", Payload: model.JSON(binding), Meta: f.Meta(f.Owner())})
			if e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				_, e = f.Store.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
					return model.Receipt{}, outbound.EnqueueNotifications(tx, f.Store.Identity().ServiceID, f.Store.Identity().Generation, c.TaskID)
				})
				if e != nil {
					t.Fatal(e)
				}
			}
			var count int
			f.Store.QueryRow(t.Context(), "SELECT count(*) FROM external_jobs").Scan(&count)
			if count != 1 {
				t.Fatal("candidate notification dedupe", count)
			}
			meta := f.Meta(f.Owner())
			meta.ExpectedVersion = task.Version
			meta.IntegratorEpoch = task.IntegratorEpoch
			meta.AcceptancePolicyVersion = task.AcceptancePolicyVersion
			if state == "accepted" {
				f.Must(f.Owner(), "acceptance.accept", task.ID, map[string]any{"candidateId": c.CandidateID}, meta)
			}
			if state == "policy-changed" {
				policy := task.AcceptancePolicy
				policy.Conditions = append(policy.Conditions, "new owner requirement")
				f.Must(f.Owner(), "acceptance.policy", task.ID, map[string]any{"policy": policy, "reason": "new requirement"}, meta)
			}
			recorder := &notificationRecorder{}
			queue := outbound.NewQueue(f.Store, map[string]outbound.Adapter{"dingtalk": recorder})
			if e = queue.RunOnce(t.Context(), time.Now()); e != nil {
				t.Fatal(e)
			}
			want := 0
			if state == "pending" {
				want = 1
			}
			if recorder.calls != want {
				t.Fatal(state, recorder.calls)
			}
			if state == "pending" && f.Task(task.ID).Phase == "done" {
				t.Fatal("platform accepted completed task")
			}
		})
	}
}
