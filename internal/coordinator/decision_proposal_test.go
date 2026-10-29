package coordinator_test

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
	"time"
)

func TestPreparedDecisionContentDoesNotGrantAuthority(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a := f.Agent(p, "integrator")
	created := f.Must(a, "task.create", "", testkit.TaskPayload(p, 0), f.Meta(a))
	task := f.Task(created.ResourceID)
	q := requestPayload(task.ID, "")
	q["blockedAssignmentIds"] = []model.ID{}
	q["blockedActions"] = []string{"external-action"}
	q["externalProposal"] = map[string]any{"action": "push", "target": "synthetic://origin/topic", "scope": "one named branch", "originalText": "允許推送此分支一次", "expiresAt": time.Now().Add(time.Hour).UTC(), "maxUses": 1}
	r := f.Must(a, "decision.request", "", q, taskMeta(f, a, task))
	var saved map[string]json.RawMessage
	if e := json.Unmarshal(r.Result, &saved); e != nil || saved["externalProposal"] == nil {
		t.Fatal("prepared target lost", e)
	}
	meta := f.Meta(f.Owner())
	meta.ExpectedVersion = 1
	answer := f.Must(f.Owner(), "decision.resolve", r.ResourceID, map[string]any{"answer": "still reviewing", "optionId": "a"}, meta)
	var result map[string]json.RawMessage
	_ = json.Unmarshal(answer.Result, &result)
	if string(result["externalAuthorization"]) != "null" && result["externalAuthorization"] != nil {
		t.Fatal("draft granted authority")
	}
}
