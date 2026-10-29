package coordinator_test

import (
	"context"
	"encoding/json"
	"testing"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
)

func TestRiskQuestionMessageKey(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	payload := testkit.TaskPayload(p, 0)
	payload.Risk = "high"
	task := f.Must(f.Owner(), "task.create", "", payload, f.Meta(f.Owner()))
	view, e := f.Core.Read(context.Background(), f.Owner(), model.Query{Kind: "decisions", ProjectID: p})
	if e != nil {
		t.Fatal(e)
	}
	var list []map[string]any
	if e = json.Unmarshal(view.Data, &list); e != nil || len(list) != 1 {
		t.Fatalf("decisions %s, %v", view.Data, e)
	}
	if list[0]["messageKey"] != "system.riskStart" {
		t.Fatalf("list messageKey %v", list[0]["messageKey"])
	}
	one, e := f.Core.Read(context.Background(), f.Owner(), model.Query{Kind: "decision", TargetID: model.ID(list[0]["id"].(string))})
	if e != nil {
		t.Fatal(e)
	}
	var detail struct {
		Request map[string]any `json:"request"`
	}
	if e = json.Unmarshal(one.Data, &detail); e != nil || detail.Request["messageKey"] != "system.riskStart" {
		t.Fatalf("detail messageKey %s, %v", one.Data, e)
	}
	needs, e := f.Core.Read(context.Background(), f.Owner(), model.Query{Kind: "needs-you"})
	if e != nil {
		t.Fatal(e)
	}
	var items []map[string]any
	if e = json.Unmarshal(needs.Data, &items); e != nil || len(items) != 1 || items[0]["messageKey"] != "system.riskStart" {
		t.Fatalf("needs-you messageKey %s, %v", needs.Data, e)
	}
	var stored string
	if e = f.Store.QueryRow(context.Background(), "SELECT body FROM decision_requests WHERE task_id=?", task.ResourceID).Scan(&stored); e != nil {
		t.Fatal(e)
	}
	var body map[string]any
	if json.Unmarshal([]byte(stored), &body); body["messageKey"] != nil {
		t.Fatal("messageKey must be derived, not stored")
	}
}
