package coordinator_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
)

type fakeInspector struct{ calls int }

func (f *fakeInspector) Inspect(context.Context, string, string) (outbound.RepositoryInspection, error) {
	f.calls++
	return outbound.RepositoryInspection{Repository: "Owner/Repo", RepositoryID: 42, Private: true, HasIssues: true}, nil
}
func TestGuidedIntegrationSecretsAndRetry(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	agent := f.Agent(p, "agent")
	inspect := &fakeInspector{}
	f.Core = coordinator.New(f.Store, nil, nil, coordinator.Options{RepositoryInspector: inspect})
	command := model.Command{Kind: "binding.inspect", Payload: model.JSON(map[string]any{"repository": "Owner/Repo", "credentialRef": "private-ref"}), Meta: f.Meta(f.Owner())}
	if _, e := f.Core.Outbound(t.Context(), agent, command); e == nil || inspect.calls != 0 {
		t.Fatal("agent reached inspector")
	}
	first, e := f.Core.Outbound(t.Context(), f.Owner(), command)
	if e != nil {
		t.Fatal(e)
	}
	retry, e := f.Core.Outbound(t.Context(), f.Owner(), command)
	if e != nil || string(first.Result) != string(retry.Result) || inspect.calls != 1 {
		t.Fatal("inspection retry changed", e, inspect.calls)
	}
	command = model.Command{Kind: "binding.credential", Payload: model.JSON(map[string]any{"kind": "dingtalk", "webhookUrl": "https://oapi.dingtalk.com/robot/send?access_token=synthetic-token", "secret": "synthetic-sign"}), Meta: f.Meta(f.Owner())}
	r, e := f.Core.Outbound(t.Context(), f.Owner(), command)
	if e != nil {
		t.Fatal(e)
	}
	r2, e := f.Core.Outbound(t.Context(), f.Owner(), command)
	if e != nil || r.ResourceID != r2.ResourceID {
		t.Fatal("secret retry changed", e)
	}
	if strings.Contains(string(model.JSON(r)), "synthetic-") {
		t.Fatal("receipt leaked secret")
	}
	raw, e := outbound.ResolveCredential(filepath.Dir(f.Store.Path))(string(r.ResourceID))
	if e != nil {
		t.Fatal(e)
	}
	var stored map[string]any
	_ = json.Unmarshal(raw, &stored)
	if stored["token"] != "synthetic-token" || stored["webhookUrl"] != nil {
		t.Fatal("stored full URL", stored)
	}
	var events string
	if e := f.Store.QueryRow(t.Context(), "SELECT coalesce(group_concat(body),'') FROM events").Scan(&events); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(events, "synthetic-token") || strings.Contains(events, "synthetic-sign") {
		t.Fatal("event leaked credential")
	}
	command.Meta = f.Meta(f.Owner())
	command.Payload = model.JSON(map[string]any{"kind": "dingtalk", "webhookUrl": "https://oapi.dingtalk.com/robot/send?access_token=synthetic-token", "token": "ambiguous", "secret": "synthetic-sign"})
	if _, e = f.Core.Outbound(t.Context(), f.Owner(), command); e == nil {
		t.Fatal("accepted ambiguous secret")
	}
}
