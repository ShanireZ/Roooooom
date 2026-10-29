package coordinator_test

import (
	"bytes"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestFinalOwnerPromotesRetainedPolicy(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "integrator")
	task := f.Must(a, "task.create", "", testkit.TaskPayload(p, 0), f.Meta(a))
	before := f.Task(task.ResourceID)
	policy := before.AcceptancePolicy
	policy.Conditions = append(policy.Conditions, "owner additionally confirms")
	m := f.Meta(f.Owner())
	m.ExpectedVersion = before.Version
	f.Must(f.Owner(), "acceptance.policy", task.ResourceID, map[string]any{"policy": policy, "reason": "I explicitly require every listed condition"}, m)
	after := f.Task(task.ResourceID)
	if after.AcceptanceConstraints.Requirements[0].ID != before.AcceptanceConstraints.Requirements[0].ID {
		t.Fatal("requirement identity changed")
	}
	policy.Conditions = policy.Conditions[1:]
	m = f.Meta(a)
	m.ExpectedVersion = after.Version
	m.IntegratorEpoch = 1
	m.AcceptancePolicyVersion = after.AcceptancePolicyVersion
	if _, e := f.Apply(a, "acceptance.policy", task.ResourceID, map[string]any{"policy": policy, "reason": "remove old condition"}, m); model.Status(e) != 403 {
		t.Fatal("agent removed condition explicitly confirmed by owner", e)
	}
}

func TestFinalFormerIntegratorCannotReviseAccepted(t *testing.T) {
	c := makeCandidate(t, "integrator")
	f := c.F
	a := c.Agent
	b := c.Acceptor
	before := f.Assignment(c.AssignmentID)
	offer := propose(t, f, a, b, c.TaskID, map[string]any{"fromSessionId": a.SessionID, "toSessionId": b.SessionID, "dispositions": []any{}, "reason": "accepted work remains historical; N=0"})
	accept(t, f, b, c.TaskID, offer)
	m := f.Meta(a)
	m.ExpectedVersion = before.Version
	m.ClaimEpoch = before.ClaimEpoch
	if _, e := f.Apply(a, "delivery.submit", c.AssignmentID, c.SubmitPayload, m); e == nil {
		t.Fatal("former integrator revised accepted claim outside worker capacity")
	}
	after := f.Assignment(c.AssignmentID)
	if after.AcceptedDeliveryID != before.AcceptedDeliveryID || after.CurrentDeliveryID != before.CurrentDeliveryID {
		t.Fatal("historical delivery changed")
	}
	var count int
	if e := f.Store.QueryRow(t.Context(), "SELECT count(*) FROM deliveries WHERE id=?", c.DeliveryID).Scan(&count); e != nil || count != 1 {
		t.Fatal("old immutable delivery lost", e)
	}
}

func TestFinalFormerIntegratorCannotReleaseAccepted(t *testing.T) {
	c := makeCandidate(t, "integrator")
	f, a, b := c.F, c.Agent, c.Acceptor
	before := f.Assignment(c.AssignmentID)
	offer := propose(t, f, a, b, c.TaskID, map[string]any{"fromSessionId": a.SessionID, "toSessionId": b.SessionID, "dispositions": []any{}, "reason": "leave accepted work historical"})
	accept(t, f, b, c.TaskID, offer)
	m := f.Meta(a)
	m.ExpectedVersion = before.Version
	m.ClaimEpoch = before.ClaimEpoch
	if _, e := f.Apply(a, "assignment.release", c.AssignmentID, map[string]any{"reason": "old claimant attempts reopening"}, m); e == nil {
		t.Fatal("former integrator reopened accepted work after leaving task")
	}
	if after := f.Assignment(c.AssignmentID); after.State != "accepted" || after.AcceptedDeliveryID != before.AcceptedDeliveryID {
		t.Fatal("historical acceptance lost")
	}
}

func TestFinalProjectMessageSenderCanRead(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b, other := f.Agent(p, "sender"), f.Agent(p, "recipient"), f.Agent(p, "unrelated")
	sent := f.Must(a, "message.send", "", map[string]any{"projectId": p, "threadId": "direct", "purpose": "discussion", "priority": "normal", "body": "private project message", "recipientMode": "participants", "recipientIds": []model.ID{b.ParticipantID}, "actionRequired": false}, f.Meta(a))
	view, e := f.Core.Read(t.Context(), a, model.Query{Kind: "message", TargetID: sent.ResourceID})
	if e != nil || !bytes.Contains(view.Data, []byte("private project message")) {
		t.Fatal("sender cannot read own message/receipts", e)
	}
	view, e = f.Core.Read(t.Context(), a, model.Query{Kind: "thread", ProjectID: p})
	if e != nil || !bytes.Contains(view.Data, []byte(sent.ResourceID)) {
		t.Fatal("sender thread omits own message", e)
	}
	if _, e = f.Core.Read(t.Context(), other, model.Query{Kind: "message", TargetID: sent.ResourceID}); model.Status(e) != 403 {
		t.Fatal("unrelated member read private message", e)
	}
	var n int
	f.Store.QueryRow(t.Context(), "SELECT count(*) FROM message_receipts WHERE message_id=? AND participant_id=?", sent.ResourceID, a.ParticipantID).Scan(&n)
	if n != 0 {
		t.Fatal("fake self receipt created")
	}
}

func TestFinalAssignmentCancellation(t *testing.T) {
	for _, owner := range []bool{false, true} {
		t.Run(map[bool]string{false: "integrator", true: "owner"}[owner], func(t *testing.T) {
			c := makeCandidate(t, "integrator")
			f := c.F
			a := c.Agent
			m := f.Meta(a)
			m.IntegratorEpoch = 1
			childPayload := testkit.AssignmentPayload(c.TaskID, c.RepoID)
			childPayload.Dependencies = []model.ID{c.AssignmentID}
			child := f.Must(a, "assignment.create", "", childPayload, m)
			// Withdraw an unfinished submitted revision; historical accepted revision survives.
			m = f.Meta(a)
			m.ExpectedVersion = f.Assignment(c.AssignmentID).Version
			m.ClaimEpoch = 1
			f.Must(a, "delivery.submit", c.AssignmentID, c.SubmitPayload, m)
			actor := a
			if owner {
				actor = f.Owner()
			}
			before := f.Assignment(c.AssignmentID)
			m = f.Meta(actor)
			m.ExpectedVersion = before.Version
			m.IntegratorEpoch = 1
			r := f.Must(actor, "assignment.update", c.AssignmentID, map[string]any{"state": "cancelled", "reason": "obsolete responsibility"}, m)
			var after model.Assignment
			json.Unmarshal(r.Result, &after)
			if after.State != "cancelled" || after.ClaimParticipantID != "" || after.ClaimSessionID != "" || after.ClaimEpoch <= before.ClaimEpoch || after.CurrentDeliveryID != before.CurrentDeliveryID {
				t.Fatal("cancel failed to fence claim/preserve history", after)
			}
			m = f.Meta(a)
			m.ExpectedVersion = after.Version
			if _, e := f.Apply(a, "assignment.claim", c.AssignmentID, struct{}{}, m); e == nil {
				t.Fatal("cancelled assignment remained claimable")
			}
			view, e := f.Core.Read(t.Context(), a, model.Query{Kind: "assignment", TargetID: child.ResourceID})
			if e != nil {
				t.Fatal(e)
			}
			var dependent model.Assignment
			json.Unmarshal(view.Data, &dependent)
			if dependent.CanStart {
				t.Fatal("cancelled dependency silently satisfied")
			}
			if f.Task(c.TaskID).Phase == "cancelled" || f.Task(c.TaskID).CurrentCandidateID != "" {
				t.Fatal("wrong task/candidate state")
			}
		})
	}
}
