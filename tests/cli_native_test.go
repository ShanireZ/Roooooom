package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/client"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
	"time"
)

func TestCLINativeOfflineReconciliation(t *testing.T) {
	root := t.TempDir()
	info, process := startService(t, root, "127.0.0.1:0")
	origin := "http://" + info["listenAddr"].(string)
	raw, e := os.ReadFile(filepath.Join(root, "activation.html"))
	if e != nil {
		t.Fatal(e)
	}
	cap := regexp.MustCompile(`name="capability" value="([^"]+)"`).FindSubmatch(raw)
	if len(cap) != 2 {
		t.Fatal("missing activation")
	}
	httpClient := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	request, _ := http.NewRequest("POST", origin+"/api/v1/setup/activate", strings.NewReader(url.Values{"capability": {string(cap[1])}}.Encode()))
	request.Header.Set("Origin", "null")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, e := httpClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 303 {
		t.Fatal("activation", response.StatusCode)
	}
	cookie := response.Cookies()[0]
	request, _ = http.NewRequest("GET", origin+"/api/v1/me", nil)
	request.AddCookie(cookie)
	response, e = httpClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	var me struct{ Result struct{ CSRF string } }
	e = json.NewDecoder(response.Body).Decode(&me)
	response.Body.Close()
	if e != nil {
		t.Fatal(e)
	}
	ownerCall := func(kind string, target model.ID, payload any) map[string]any {
		t.Helper()
		body := model.JSON(map[string]any{"serviceId": info["serviceId"], "datasetGeneration": info["datasetGeneration"], "clientRequestId": string((model.RandomIDs{}).NewID()), "targetId": target, "payload": payload})
		r, _ := http.NewRequest("POST", origin+"/api/v1/commands/"+kind, bytes.NewReader(body))
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", me.Result.CSRF)
		response, e := httpClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		var result struct{ Result map[string]any }
		if response.StatusCode != 200 {
			b, _ := io.ReadAll(response.Body)
			t.Fatal(kind, response.StatusCode, string(b))
		}
		if e = json.NewDecoder(response.Body).Decode(&result); e != nil {
			t.Fatal(e)
		}
		return result.Result
	}
	project := ownerCall("project.create", "", map[string]any{"name": "CLI synthetic", "repositories": []any{map[string]any{"name": "repo", "reference": "synthetic://repo"}}})
	projectID := model.ID(project["id"].(string))
	ownerCall("enrollment.enable", projectID, struct{}{})
	cliCall := func(input any, expected int, args ...string) map[string]any {
		t.Helper()
		args = append(args, "--data-dir", root, "--json")
		cmd := exec.Command(binary(t), args...)
		hideProcess(cmd)
		if input != nil {
			cmd.Stdin = bytes.NewReader(model.JSON(input))
		}
		output, e := cmd.CombinedOutput()
		code := 0
		if e != nil {
			var exit *exec.ExitError
			if !errors.As(e, &exit) {
				t.Fatal(e)
			}
			code = exit.ExitCode()
		}
		if code != expected {
			t.Fatal(args, code, string(output))
		}
		if bytes.Contains(output, []byte(cookie.Value)) {
			t.Fatal("owner token printed")
		}
		var body map[string]any
		if e = json.Unmarshal(output, &body); e != nil {
			t.Fatal("non-JSON CLI output", string(output))
		}
		return body
	}
	connected := cliCall(nil, 0, "connect", "--profile", "integrator", "--project", string(projectID))
	if connected["mode"] != "online" {
		t.Fatal(connected)
	}
	created := cliCall(map[string]any{"payload": testkit.TaskPayload(projectID, 0)}, 0, "task", "create", "--profile", "integrator")
	receipt := created["receipt"].(map[string]any)
	task := receipt["result"].(map[string]any)
	taskID := model.ID(task["id"].(string))
	originalSession := connected["sessionId"]
	_ = process.Process.Kill()
	_ = process.Wait()
	offline := cliCall(nil, 0, "connect", "--profile", "integrator", "--project", string(projectID))
	if offline["mode"] != "offline" {
		t.Fatal(offline)
	}
	cliCall(map[string]any{"targetId": taskID, "expectedVersion": 1, "integratorEpoch": 1, "specVersion": 1, "acceptancePolicyVersion": 1, "payload": map[string]any{"candidateId": "offline-candidate"}}, 1, "task", "complete", "--profile", "integrator")
	cliCall(nil, 0, "connect", "--profile", "first-offline", "--project", string(projectID))
	cliCall(map[string]any{"payload": testkit.TaskPayload(projectID, 0)}, 1, "task", "create", "--profile", "first-offline")
	_, _ = startService(t, root, "127.0.0.1:0")
	reconnected := cliCall(nil, 0, "connect", "--profile", "integrator", "--project", string(projectID))
	if reconnected["sessionId"] == originalSession {
		t.Fatal("connect silently reused a work session")
	}
	reconciled := cliCall(nil, 0, "offline", "reconcile", "--profile", "integrator")
	results := reconciled["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["state"] != "candidate" {
		t.Fatal(reconciled)
	}
	read := cliCall(nil, 0, "task", "get", "--profile", "integrator", "--target", string(taskID))
	current := read["result"].(map[string]any)
	if current["phase"] == "done" || current["integratorSessionId"] != originalSession {
		t.Fatal("reconcile gained control or completion", current)
	}
	cliCall(nil, 0, "connect", "--profile", "first-offline", "--project", string(projectID))
	cliCall(nil, 0, "offline", "reconcile", "--profile", "first-offline")
	rootClient, _ := client.ProfileRoot(root, "integrator")
	profile, e := client.LoadProfile(rootClient)
	if e != nil {
		t.Fatal(e)
	}
	credential, e := os.ReadFile(filepath.Join(rootClient, profile.CredentialRef))
	if e != nil {
		t.Fatal(e)
	}
	var saved struct{ Token string }
	_ = json.Unmarshal(credential, &saved)
	status := cliCall(nil, 0, "status", "--profile", "integrator")
	if strings.Contains(string(model.JSON(status)), saved.Token) || status["pendingDrafts"] != float64(0) {
		t.Fatal("credential leak or outbox loss", status)
	}
}
