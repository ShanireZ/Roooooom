package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"roooooom.local/roooooom/internal/control"
	"roooooom.local/roooooom/internal/launcher"
	"testing"
	"time"
)

func TestLauncherPrivateControlRealBinary(t *testing.T) {
	root := t.TempDir()
	info, process := startService(t, root, "127.0.0.1:0")
	origin := "http://" + info["listenAddr"].(string)
	raw, e := os.ReadFile(filepath.Join(root, "control.json"))
	if e != nil {
		t.Fatal(e)
	}
	var file control.File
	if e = json.Unmarshal(raw, &file); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains([]byte(origin), []byte(file.Token)) {
		t.Fatal("control secret in URL")
	}
	request, _ := http.NewRequest("POST", origin+"/api/v1/control/stop", bytes.NewReader([]byte(`{"serviceId":"wrong","datasetGeneration":"wrong"}`)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer agent-token-cannot-stop")
	response, e := http.DefaultClient.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("agent may stop", response.StatusCode)
	}
	client, e := launcher.Connect(root, filepath.Dir(binary(t)))
	if e != nil {
		t.Fatal(e)
	}
	proof, e := client.Probe(context.Background())
	if e != nil || !proof.NeedsActivation {
		t.Fatal(proof, e)
	}
	path, e := client.Activate(context.Background())
	if e != nil || path != filepath.Join(root, "activation.html") {
		t.Fatal(path, e)
	}
	if e = client.Stop(context.Background()); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case e = <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("writer still running")
	}
}

func TestLauncherOwnChildSurvivesLauncherExit(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "套件 空間")
	if e := os.MkdirAll(bundle, 0700); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(t.TempDir(), "協作 空間")
	core := filepath.Join(bundle, launcher.CoreName())
	raw, e := os.ReadFile(binary(t))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(core, raw, 0700); e != nil {
		t.Fatal(e)
	}
	name := "roooooom-launcher"
	if filepath.Ext(core) == ".exe" {
		name += ".exe"
	}
	entry := filepath.Join(bundle, name)
	build := exec.Command("go", "build", "-o", entry, "./cmd/roooooom-launcher")
	build.Dir = ".."
	hideProcess(build)
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	t.Cleanup(func() {
		if client, e := launcher.Connect(root, bundle); e == nil {
			client.Stop(context.Background())
		}
	})
	run := func(args ...string) {
		cmd := exec.Command(entry, append([]string{"--data-dir", root, "--headless"}, args...)...)
		hideProcess(cmd)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatal(e, string(out))
		}
	}
	run("--listen", "127.0.0.1:0")
	first, e := control.Read(root)
	if e != nil {
		t.Fatal(e)
	}
	client, e := launcher.Connect(root, bundle)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = client.Probe(t.Context()); e != nil {
		t.Fatal("core exited with launcher", e)
	}
	run("--listen", "127.0.0.1:0")
	second, e := control.Read(root)
	if e != nil || first != second {
		t.Fatal("second launch replaced running core", e)
	}
	response, e := http.Get("http://" + first.Address + "/")
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if _, e = client.Probe(t.Context()); e != nil {
		t.Fatal("closing UI connection stopped service", e)
	}
	run("--stop")
	if _, e = client.Probe(t.Context()); e == nil {
		t.Fatal("explicit stop left writer running")
	}
}
