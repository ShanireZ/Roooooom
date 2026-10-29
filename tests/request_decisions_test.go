package tests

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestRequestDecisionHTTPBoundary(t *testing.T) {
	f := newAuthFixture(t)
	f.handler = httpapi.New(f.core, f.ids, httpapi.Options{AllowedHost: "127.0.0.1:7337"})
	owner, e := f.ids.Authenticate(t.Context(), f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	identity := f.store.Identity()
	meta := func(session model.ID) model.Mutation {
		return model.Mutation{ServiceID: identity.ServiceID, Generation: identity.Generation, ClientRequestID: string((model.RandomIDs{}).NewID()), SessionID: session, SpecVersion: 1, AcceptancePolicyVersion: 1}
	}
	project, e := f.core.Apply(t.Context(), owner, model.Command{Kind: "project.create", Payload: model.JSON(coordinator.ProjectPayload{Name: "HTTP intake", Repositories: []coordinator.RepositoryPayload{}}), Meta: meta("")})
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(t.Context(), owner, project.ResourceID)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.ids.Enroll(t.Context(), cap, "intake-http", "preparer")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"request.create", "request.cancel", "request.assign"} {
		response := f.request("POST", "/api/v1/commands/"+kind, "{}", credentials.Token, "")
		if response.Code != 403 {
			t.Fatalf("%s decoded an agent's owner-only request: %d", kind, response.Code)
		}
	}
	send := func(kind string, target model.ID, payload any, m model.Mutation, token string) json.RawMessage {
		t.Helper()
		origin := ""
		if token == "owner" {
			origin = "http://127.0.0.1:7337"
		}
		raw := model.JSON(struct {
			model.Mutation
			TargetID model.ID `json:"targetId,omitempty"`
			Payload  any      `json:"payload"`
		}{m, target, payload})
		response := f.request("POST", "/api/v1/commands/"+kind, string(raw), token, origin)
		if response.Code != 200 {
			t.Fatalf("%s: %d %s", kind, response.Code, response.Body.String())
		}
		var envelope struct{ Result json.RawMessage }
		if e := json.Unmarshal(response.Body.Bytes(), &envelope); e != nil {
			t.Fatal(e)
		}
		return envelope.Result
	}
	var intake model.WorkRequest
	raw := send("request.create", "", map[string]any{"projectId": project.ResourceID, "text": "Clarify and then implement this request"}, meta(""), "owner")
	if e = json.Unmarshal(raw, &intake); e != nil {
		t.Fatal(e)
	}
	m := meta(credentials.SessionID)
	m.ExpectedVersion = 1
	raw = send("request.claim", intake.ID, map[string]any{}, m, credentials.Token)
	json.Unmarshal(raw, &intake)
	question := map[string]any{"requestId": intake.ID, "scopeKey": "display", "question": "Which format?", "reason": "Choose before task creation", "factRefs": []string{}, "options": []any{map[string]string{"id": "a", "label": "A", "impact": "Simple"}, map[string]string{"id": "b", "label": "B", "impact": "Detailed"}}, "recommendation": "a", "recommendationReason": "Simple", "impact": "Task scope", "blockedAssignmentIds": []string{}, "blockedActions": []string{"request-convert"}, "canContinue": []string{}, "noDecisionBehavior": "Wait before conversion"}
	m = meta(credentials.SessionID)
	m.ExpectedVersion = intake.Version
	m.ClaimEpoch = intake.PreparationEpoch
	for _, both := range []bool{false, true} {
		bad := map[string]any{}
		for k, v := range question {
			bad[k] = v
		}
		if both {
			bad["taskId"] = "not-a-task"
		} else {
			delete(bad, "requestId")
		}
		wire := model.JSON(struct {
			model.Mutation
			Payload any `json:"payload"`
		}{m, bad})
		response := f.request("POST", "/api/v1/commands/decision.request", string(wire), credentials.Token, "")
		if response.Code != 400 {
			t.Fatal("invalid subject accepted", response.Code, response.Body.String())
		}
	}
	var pending struct {
		ID      model.ID
		Version int64
	}
	raw = send("decision.request", "", question, m, credentials.Token)
	json.Unmarshal(raw, &pending)
	var taskCount int
	if e = f.store.QueryRow(t.Context(), "SELECT count(*) FROM tasks").Scan(&taskCount); e != nil || taskCount != 0 {
		t.Fatal("HTTP created placeholder task", taskCount, e)
	}
	m = meta("")
	m.ExpectedVersion = pending.Version
	send("decision.resolve", pending.ID, map[string]any{"answer": "A", "optionId": "a"}, m, "owner")
	response := f.request("GET", "/api/v1/queries/request?targetId="+string(intake.ID), "", credentials.Token, "")
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	var view struct{ Result model.WorkRequest }
	json.Unmarshal(response.Body.Bytes(), &view)
	intake = view.Result
	m = meta(credentials.SessionID)
	m.ExpectedVersion = intake.Version
	m.ClaimEpoch = intake.PreparationEpoch
	m.DecisionVersions = map[model.ID]int64{pending.ID: 1}
	raw = send("request.convert", intake.ID, map[string]any{"task": testkit.TaskPayload(project.ResourceID, 0)}, m, credentials.Token)
	var converted struct{ Task model.Task }
	if e = json.Unmarshal(raw, &converted); e != nil || converted.Task.SourceRequestID != intake.ID || converted.Task.DecisionVersions[pending.ID] != 1 {
		t.Fatal("HTTP source chain lost", e)
	}
}
