package tests

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
)

func TestHTTPFormalHumanAcceptance(t *testing.T) {
	f := newAuthFixture(t)
	ctx := t.Context()
	f.handler = httpapi.New(f.core, f.ids, httpapi.Options{AllowedHost: "127.0.0.1:7337", BodyLimitBytes: 1024 * 1024})
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	call := func(token, kind string, target model.ID, payload any, meta model.Mutation) map[string]any {
		t.Helper()
		body := map[string]any{}
		if e = json.Unmarshal(model.JSON(meta), &body); e != nil {
			t.Fatal(e)
		}
		body["targetId"] = target
		body["payload"] = payload
		origin := ""
		if token == "owner" {
			origin = "http://127.0.0.1:7337"
		}
		w := f.request("POST", "/api/v1/commands/"+kind, string(model.JSON(body)), token, origin)
		if w.Code != 200 {
			t.Fatal(kind, w.Code, w.Body.String())
		}
		var response struct{ Result map[string]any }
		if e = json.Unmarshal(w.Body.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		return response.Result
	}
	identity := f.store.Identity()
	meta := func(session model.ID, version, epoch int64) model.Mutation {
		return model.Mutation{ServiceID: identity.ServiceID, Generation: identity.Generation, ClientRequestID: string((model.RandomIDs{}).NewID()), SessionID: session, ExpectedVersion: version, IntegratorEpoch: epoch, ClaimEpoch: epoch, SpecVersion: 1, AcceptancePolicyVersion: 1, DecisionVersions: map[model.ID]int64{}}
	}
	project := call("owner", "project.create", "", map[string]any{"name": "HTTP synthetic", "repositories": []any{map[string]any{"name": "repo", "reference": "synthetic://repo"}}}, meta("", 0, 0))
	projectID := model.ID(project["id"].(string))
	var repo model.ID
	if e = f.store.QueryRow(ctx, "SELECT id FROM repositories WHERE project_id=?", projectID).Scan(&repo); e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(ctx, owner, projectID)
	if e != nil {
		t.Fatal(e)
	}
	agent, e := f.ids.Enroll(ctx, cap, "worker", "integrator")
	if e != nil {
		t.Fatal(e)
	}
	payload := testkit.TaskPayload(projectID, 0)
	payload.AcceptancePolicy.Mode = "human"
	task := call(agent.Token, "task.create", "", payload, meta(agent.SessionID, 0, 0))
	taskID := model.ID(task["id"].(string))
	current := func() model.Task {
		view, e := f.core.Read(ctx, owner, model.Query{Kind: "task", TargetID: taskID})
		if e != nil {
			t.Fatal(e)
		}
		var task model.Task
		if e = json.Unmarshal(view.Data, &task); e != nil {
			t.Fatal(e)
		}
		return task
	}
	assignment := call(agent.Token, "assignment.create", "", testkit.AssignmentPayload(taskID, repo), meta(agent.SessionID, 0, 1))
	assignmentID := model.ID(assignment["id"].(string))
	call(agent.Token, "assignment.claim", assignmentID, struct{}{}, meta(agent.SessionID, 1, 1))
	delivery := call(agent.Token, "delivery.submit", assignmentID, map[string]any{"repoId": repo, "baseSha": strings.Repeat("a", 40), "headSha": strings.Repeat("b", 40), "branchRef": "synthetic", "workspaceRef": "synthetic", "summary": "change", "evidenceRefs": []string{"synthetic://unit"}, "unknowns": []string{}, "artifactRefs": []any{}}, meta(agent.SessionID, 2, 1))
	deliveryID := model.ID(delivery["id"].(string))
	call(agent.Token, "delivery.review", assignmentID, map[string]any{"deliveryRevisionId": deliveryID, "disposition": "accepted", "reason": "integrator checked references"}, meta(agent.SessionID, 3, 1))
	attempt := call(agent.Token, "integration.record", taskID, map[string]any{"deliveryRevisionIds": []model.ID{deliveryID}, "heads": []any{map[string]any{"repoId": repo, "baseSha": strings.Repeat("a", 40), "headSha": strings.Repeat("b", 40)}}, "checks": []any{map[string]any{"condition": "tests on exact head", "outcome": "passed", "headShas": map[model.ID]string{repo: strings.Repeat("b", 40)}, "evidenceRefs": []string{"synthetic://integration"}}}, "summary": "reported checks", "result": "passed", "excludedAssignments": []any{}, "untested": []string{}}, meta(agent.SessionID, current().Version, 1))
	candidate := call(agent.Token, "acceptance.candidate", taskID, map[string]any{"integrationId": attempt["id"]}, meta(agent.SessionID, current().Version, 1))
	candidateID := model.ID(candidate["id"].(string))
	acceptMeta := meta(agent.SessionID, current().Version, 1)
	body := map[string]any{}
	_ = json.Unmarshal(model.JSON(acceptMeta), &body)
	body["targetId"] = taskID
	body["payload"] = map[string]any{"candidateId": candidateID}
	if w := f.request("POST", "/api/v1/commands/acceptance.accept", string(model.JSON(body)), agent.Token, ""); w.Code != 403 {
		t.Fatal("agent forged human acceptance", w.Code)
	}
	call("owner", "acceptance.accept", taskID, map[string]any{"candidateId": candidateID}, meta("", current().Version, 1))
	call(agent.Token, "task.complete", taskID, map[string]any{"candidateId": candidateID}, meta(agent.SessionID, current().Version, 1))
	if current().Phase != "done" {
		t.Fatal("formal human flow did not complete")
	}
}
