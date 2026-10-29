package tests

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"roooooom.local/roooooom/internal/cli"
	"roooooom.local/roooooom/internal/client"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestNativeRequestAliasesUseRealProtocol(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	intake := f.Must(f.Owner(), "request.create", "", map[string]any{"projectId": p, "text": "Native intake handoff"}, f.Meta(f.Owner()))
	cap, e := f.Identities.EnableEnrollment(t.Context(), f.Owner(), p)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.Identities.Enroll(t.Context(), cap, "cli-intake", "cli-intake")
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: server.Listener.Addr().String()})
	server.Start()
	defer server.Close()
	data := t.TempDir()
	root, e := client.ProfileRoot(data, "cli-intake")
	if e != nil {
		t.Fatal(e)
	}
	if e = privatefs.WriteAtomic(root, "credentials/session.json", model.JSON(credentials)); e != nil {
		t.Fatal(e)
	}
	identity := f.Store.Identity()
	profile := client.Profile{Root: root, Handle: "cli-intake", ServiceURL: server.URL, ServiceID: identity.ServiceID, Generation: identity.Generation, ProjectID: p, ParticipantID: credentials.ParticipantID, SessionID: credentials.SessionID, CredentialRef: "credentials/session.json", Mode: "online"}
	if e = client.SaveProfile(profile); e != nil {
		t.Fatal(e)
	}
	run := func(action string, body []byte) []byte {
		t.Helper()
		args := []string{"request", action, "--data-dir", data, "--profile", "cli-intake", "--json"}
		if action == "get" {
			args = append(args, "--target", string(intake.ResourceID))
		}
		var out, errOut bytes.Buffer
		if code := cli.Run(t.Context(), args, bytes.NewReader(body), &out, &errOut); code != 0 {
			t.Fatalf("%s exit%d %s %s", action, code, out.String(), errOut.String())
		}
		return out.Bytes()
	}
	var listed struct{ Result []model.WorkRequest }
	if e = json.Unmarshal(run("list", nil), &listed); e != nil || len(listed.Result) != 1 {
		t.Fatal(e, listed)
	}
	var got struct{ Result model.WorkRequest }
	if e = json.Unmarshal(run("get", nil), &got); e != nil || got.Result.ID != intake.ResourceID {
		t.Fatal(e)
	}
	run("claim", model.JSON(map[string]any{"targetId": intake.ResourceID, "expectedVersion": got.Result.Version, "claimEpoch": 0, "payload": map[string]any{}}))
	if e = json.Unmarshal(run("get", nil), &got); e != nil || got.Result.State != "preparing" || got.Result.PreparerSessionID != credentials.SessionID {
		t.Fatal(e, got)
	}
}
