package coordinator_test

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestOwnerViewsAndAssignmentCanStart(t *testing.T) {
	f := testkit.New(t)
	project, repo := f.Project()
	a := f.Agent(project, "visible-participant")
	task := f.Must(a, "task.create", "", testkit.TaskPayload(project, 1), f.Meta(a))
	meta := f.Meta(a)
	meta.IntegratorEpoch = 1
	assignment := f.Must(a, "assignment.create", "", testkit.AssignmentPayload(task.ResourceID, repo), meta)
	view, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "participants", ProjectID: project})
	if e != nil {
		t.Fatal(e)
	}
	var participants []map[string]any
	json.Unmarshal(view.Data, &participants)
	if len(participants) != 1 || participants[0]["name"] != "visible-participant" {
		t.Fatal(participants)
	}
	all, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "participants"})
	if e != nil {
		t.Fatal("owner overview cannot identify participants", e)
	}
	if string(all.Data) != string(view.Data) {
		t.Fatal("owner overview participant scope disagrees")
	}
	taskView, e := f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "task", TargetID: task.ResourceID})
	if e != nil {
		t.Fatal(e)
	}
	var members struct {
		ParticipantIDs []model.ID `json:"participantIds"`
	}
	json.Unmarshal(taskView.Data, &members)
	if len(members.ParticipantIDs) != 1 || members.ParticipantIDs[0] != a.ParticipantID {
		t.Fatal("board cannot filter task participants", members)
	}
	other, _ := f.Project()
	foreign := f.Agent(other, "foreign")
	if _, e = f.Core.Read(t.Context(), foreign, model.Query{Kind: "participants", ProjectID: project}); model.Status(e) != 403 {
		t.Fatal("cross-project participants visible", e)
	}
	current := f.Task(task.ResourceID)
	meta = f.Meta(f.Owner())
	meta.ExpectedVersion = current.Version
	f.Must(f.Owner(), "task.cancel", task.ResourceID, map[string]any{"reason": "explicit synthetic cancellation"}, meta)
	if got := f.Assignment(assignment.ResourceID); got.CanStart {
		t.Fatal("cancelled task assignment could start")
	}
	view, e = f.Core.Read(t.Context(), f.Owner(), model.Query{Kind: "assignments", TargetID: task.ResourceID})
	if e != nil {
		t.Fatal(e)
	}
	var assignments []model.Assignment
	json.Unmarshal(view.Data, &assignments)
	if len(assignments) != 1 || assignments[0].CanStart {
		t.Fatal("bulk assignment view disagrees with task gate")
	}
}
