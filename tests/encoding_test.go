package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"roooooom.local/roooooom/internal/client"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/launcher"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestHTTPRejectsNonUTF8BeforeTaskCommit(t *testing.T) {
	f := testkit.New(t)
	project, _ := f.Project()
	cap, e := f.Identities.EnableEnrollment(t.Context(), f.Owner(), project)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.Identities.Enroll(t.Context(), cap, "encoding", "encoding")
	if e != nil {
		t.Fatal(e)
	}
	actor, e := f.Identities.Authenticate(t.Context(), credentials.Token)
	if e != nil {
		t.Fatal(e)
	}
	payload := testkit.TaskPayload(project, 0)
	payload.Goal = "進度"
	raw := model.JSON(struct {
		model.Mutation
		Payload model.TaskCreate `json:"payload"`
	}{f.Meta(actor), payload})
	raw = bytes.Replace(raw, []byte("進度"), []byte{0xdf, 0x4d, 0xb6, 0xc8}, 1)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7337/api/v1/commands/task.create", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+credentials.Token)
	response := httptest.NewRecorder()
	handler := httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: "127.0.0.1:7337"})
	handler.ServeHTTP(response, request)
	if response.Code != 400 {
		t.Fatalf("malformed UTF8 accepted status=%d body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("invalid-utf8")) {
		t.Fatal("encoding rejection is not actionable", response.Body.String())
	}
	var count int
	if e = f.Store.QueryRow(t.Context(), "SELECT count(*) FROM tasks WHERE project_id=?", project).Scan(&count); e != nil || count != 0 {
		t.Fatal("malformed command committed a task", e)
	}
}

func TestArtifactRejectsNonUTF8BeforeOutbox(t *testing.T) {
	bundle, _ := artifact(t)
	data := t.TempDir()
	root, e := client.ProfileRoot(data, "artifact-encoding")
	if e != nil {
		t.Fatal(e)
	}
	profile := client.Profile{Root: root, Handle: "artifact-encoding", ProjectID: "synthetic", Mode: "offline", Provisional: true}
	if e = client.SaveProfile(profile); e != nil {
		t.Fatal(e)
	}
	payload := testkit.TaskPayload("synthetic", 0)
	payload.Goal = "進度"
	raw := model.JSON(map[string]any{"payload": payload})
	raw = bytes.Replace(raw, []byte("進度"), []byte{0xdf, 0x4d, 0xb6, 0xc8}, 1)
	command := exec.Command(filepath.Join(bundle, launcher.CoreName()), "apply", "task.create", "--data-dir", data, "--profile", "artifact-encoding", "--json")
	command.Env = onlySystemEnv(t)
	command.Stdin = bytes.NewReader(raw)
	hideProcess(command)
	output, e := command.CombinedOutput()
	if e == nil {
		t.Fatal("malformed command succeeded")
	}
	var response struct {
		Error struct {
			Code   string
			Status int
		}
		DraftID string `json:"draftId"`
	}
	if e = json.Unmarshal(output, &response); e != nil {
		t.Fatal(e)
	}
	c := client.New(profile, nil)
	defer c.Close()
	pending, e := c.Pending(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	if response.Error.Code != "invalid-utf8" || response.Error.Status != 400 || response.DraftID != "" || len(pending) != 0 {
		t.Fatal("artifact lost or staged malformed input", string(output))
	}
}
