package tests

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var buildOnce sync.Once
var testBinary string
var buildFailure error

func binary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "roooooom-test-binary-")
		if err != nil {
			buildFailure = err
			return
		}
		name := "roooooom"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		testBinary = filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", testBinary, "./cmd/roooooom")
		cmd.Dir = ".."
		hideProcess(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildFailure = &buildError{err, string(out)}
		}
	})
	if buildFailure != nil {
		t.Fatal(buildFailure)
	}
	return testBinary
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.output }
func TestMain(m *testing.M) {
	// A bad flag parser must never fall through into the developer's real profile.
	profile, err := os.MkdirTemp("", "roooooom-test-profile-")
	if err != nil {
		os.Exit(1)
	}
	_ = os.Setenv("LOCALAPPDATA", profile)
	_ = os.Setenv("APPDATA", profile)
	code := m.Run()
	if testBinary != "" {
		clean := filepath.Clean(filepath.Dir(testBinary))
		if filepath.Dir(clean) == filepath.Clean(os.TempDir()) && strings.HasPrefix(filepath.Base(clean), "roooooom-test-binary-") {
			_ = os.RemoveAll(clean)
		} else {
			code = 1
		}
	}
	clean := filepath.Clean(profile)
	if filepath.Dir(clean) == filepath.Clean(os.TempDir()) && strings.HasPrefix(filepath.Base(clean), "roooooom-test-profile-") {
		_ = os.RemoveAll(clean)
	} else {
		code = 1
	}
	os.Exit(code)
}

func startService(t *testing.T, root, address string) (map[string]any, *exec.Cmd) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binary(t), "serve", "--data-dir", root, "--listen", address)
	hideProcess(cmd)
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	path := filepath.Join(root, "discovery.json")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		bytes, err := os.ReadFile(path)
		if err == nil {
			var info map[string]any
			if json.Unmarshal(bytes, &info) == nil {
				address, ok := info["listenAddr"].(string)
				if ok {
					client := http.Client{Timeout: 100 * time.Millisecond}
					response, requestErr := client.Get("http://" + address + "/identity")
					if requestErr == nil {
						var identity map[string]any
						decodeErr := json.NewDecoder(response.Body).Decode(&identity)
						response.Body.Close()
						if response.StatusCode == 200 && decodeErr == nil && identity["serviceId"] == info["serviceId"] {
							return info, cmd
						}
					}
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("real binary did not publish discovery")
	return nil, nil
}

func TestServiceIdentityPersistsInSQLite(t *testing.T) {
	root := t.TempDir()
	first, process := startService(t, root, "127.0.0.1:0")
	if _, err := os.Stat(filepath.Join(root, "state.sqlite")); err != nil {
		t.Fatal("real service must open SQLite:", err)
	}
	if _, err := os.Stat(filepath.Join(root, "identity.json")); !os.IsNotExist(err) {
		t.Fatal("identity must have only one durable authority")
	}
	if len(first["serviceId"].(string)) != 43 {
		t.Fatal("invalid random identity")
	}
	_ = process.Process.Kill()
	_ = process.Wait()
	second, _ := startService(t, root, "127.0.0.1:0")
	if second["serviceId"] != first["serviceId"] || second["datasetGeneration"] != first["datasetGeneration"] {
		t.Fatal("restart changed durable identity")
	}
}

func TestHealthRealBinary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Unicode 空白")
	info, _ := startService(t, root, "127.0.0.1:0")
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + info["listenAddr"].(string) + "/identity")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || body["serviceId"] != info["serviceId"] || body["protocolVersion"] != float64(1) {
		t.Fatal(body)
	}
}

func TestSecondWriterRejected(t *testing.T) {
	root := t.TempDir()
	_, _ = startService(t, root, "127.0.0.1:0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary(t), "serve", "--data-dir", root, "--listen", "127.0.0.1:0")
	hideProcess(cmd)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("second service obtained the writer")
	}
	if ctx.Err() != nil {
		t.Fatal("second writer blocked instead of rejecting")
	}
	if !strings.Contains(string(output), "already running") {
		t.Fatalf("wrong failure: %s", output)
	}
}

func TestOccupiedPortAndNonLoopbackRejected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	for _, address := range []string{ln.Addr().String(), "0.0.0.0:0"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, binary(t), "serve", "--data-dir", t.TempDir(), "--listen", address)
		hideProcess(cmd)
		_, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			t.Fatal("accepted unavailable/non-loopback address", address)
		}
	}
}
