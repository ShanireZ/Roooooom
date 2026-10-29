package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"roooooom.local/roooooom/internal/client"
	"roooooom.local/roooooom/internal/launcher"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"runtime"
	"strings"
	"testing"
	"time"
)

type bundleManifest struct {
	Format            int               `json:"format"`
	Target            string            `json:"target"`
	Commit            string            `json:"commit"`
	SourceFingerprint string            `json:"sourceFingerprint"`
	CGOEnabled        bool              `json:"cgoEnabled"`
	Supported         bool              `json:"supported"`
	Files             map[string]string `json:"files"`
	UIFiles           map[string]string `json:"uiFiles"`
	SourceHashes      map[string]string `json:"sourceHashes"`
}

func artifact(t *testing.T) (string, bundleManifest) {
	t.Helper()
	if _, e := os.Stat("../packaging/platforms.json"); e != nil {
		t.Fatal("platform authority missing", e)
	}
	root := os.Getenv("ROOOOOOM_TEST_BUNDLE")
	if root == "" {
		t.Skip("artifact cases unverified without ROOOOOOM_TEST_BUNDLE; run the packaging gate explicitly")
	}
	root, e := filepath.Abs(root)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(root, "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m bundleManifest
	if e = json.Unmarshal(raw, &m); e != nil {
		t.Fatal(e)
	}
	if m.Target != runtime.GOOS+"-"+runtime.GOARCH {
		t.Fatal("native artifact required", m.Target)
	}
	return root, m
}
func onlySystemEnv(t *testing.T) []string {
	t.Helper()
	path := "/usr/bin:/bin"
	if runtime.GOOS == "windows" {
		path = filepath.Join(os.Getenv("SystemRoot"), "System32")
	}
	values := []string{"PATH=" + path, "LOCALAPPDATA=" + t.TempDir(), "APPDATA=" + t.TempDir()}
	for _, key := range []string{"SystemRoot", "WINDIR", "TEMP", "TMP"} {
		if value := os.Getenv(key); value != "" {
			values = append(values, key+"="+value)
		}
	}
	return values
}
func checksum(t *testing.T, path string) string {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type ownArtifactProcess struct {
	done    chan struct{}
	cmd     *exec.Cmd
	err     error
	address string
}

func artifactService(t *testing.T, core, root string) *ownArtifactProcess {
	t.Helper()
	p := &ownArtifactProcess{done: make(chan struct{}), cmd: exec.Command(core, "serve", "--data-dir", root, "--listen", "127.0.0.1:0")}
	p.cmd.Env = onlySystemEnv(t)
	hideProcess(p.cmd)
	if e := p.cmd.Start(); e != nil {
		t.Fatal(e)
	}
	go func() { p.err = p.cmd.Wait(); close(p.done) }()
	t.Cleanup(func() {
		select {
		case <-p.done:
			return
		default:
			_ = p.cmd.Process.Kill()
			<-p.done
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		raw, e := os.ReadFile(filepath.Join(root, "discovery.json"))
		if e == nil {
			var d struct {
				Address string `json:"listenAddr"`
			}
			if json.Unmarshal(raw, &d) == nil {
				c := http.Client{Timeout: 100 * time.Millisecond}
				r, e := c.Get("http://" + d.Address + "/identity")
				if e == nil {
					r.Body.Close()
					if r.StatusCode == 200 {
						p.address = "http://" + d.Address
						return p
					}
				}
			}
		}
		select {
		case <-p.done:
			t.Fatal("artifact exited before readiness", p.err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("artifact did not start")
	return p
}
func TestBundleNoDevRuntimeRequired(t *testing.T) {
	root, m := artifact(t)
	if m.CGOEnabled || m.Format != 1 || m.SourceFingerprint == "" {
		t.Fatal("incomplete standalone manifest", m)
	}
	if len(m.SourceHashes) == 0 {
		t.Fatal("source fingerprint inventory missing")
	}
	for name, expected := range m.SourceHashes {
		if filepath.IsAbs(name) || strings.Contains(name, "..") || strings.Contains(name, "\\") {
			t.Fatal("unsafe source inventory")
		}
		raw, e := os.ReadFile(filepath.Join("..", filepath.FromSlash(name)))
		if e != nil {
			t.Fatal(e)
		}
		h := sha256.Sum256(bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n")))
		if hex.EncodeToString(h[:]) != expected {
			t.Fatal("artifact no longer corresponds to current source", name)
		}
	}
	for name, expected := range m.Files {
		if filepath.IsAbs(name) || strings.Contains(name, "..") || strings.Contains(name, "\\") {
			t.Fatal("unsafe manifest file")
		}
		if checksum(t, filepath.Join(root, filepath.FromSlash(name))) != expected {
			t.Fatal("artifact checksum mismatch", name)
		}
	}
	core := filepath.Join(root, launcher.CoreName())
	for _, name := range []string{launcher.CoreName(), strings.Replace(launcher.CoreName(), "roooooom", "roooooom-launcher", 1)} {
		b, e := buildinfo.ReadFile(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		settings := map[string]string{}
		for _, s := range b.Settings {
			settings[s.Key] = s.Value
		}
		if settings["CGO_ENABLED"] != "0" || settings["GOOS"] != runtime.GOOS || settings["GOARCH"] != runtime.GOARCH {
			t.Fatal("runtime/native dependencies", settings)
		}
		argv := []string{"version", "--json"}
		if strings.Contains(name, "launcher") {
			argv = []string{"--version"}
		}
		cmd := exec.Command(filepath.Join(root, name), argv...)
		cmd.Env = onlySystemEnv(t)
		hideProcess(cmd)
		raw, e := cmd.CombinedOutput()
		if e != nil || !bytes.Contains(raw, []byte(m.SourceFingerprint)) {
			t.Fatal("native metadata without SDK failed", name, e)
		}
	}
	if runtime.GOOS == "windows" {
		p, e := pe.Open(filepath.Join(root, "roooooom-launcher.exe"))
		if e != nil {
			t.Fatal(e)
		}
		defer p.Close()
		header, ok := p.OptionalHeader.(*pe.OptionalHeader64)
		if !ok || header.Subsystem != 2 {
			t.Fatal("download entry opens a console")
		}
	}
	cmd := exec.Command(core, "version", "--json")
	cmd.Env = onlySystemEnv(t)
	hideProcess(cmd)
	raw, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatal("version needs a development runtime", e)
	}
	if !bytes.Contains(raw, []byte(m.SourceFingerprint)) {
		t.Fatal("binary provenance differs")
	}
	dataRoot := filepath.Join(t.TempDir(), "獨立 資料")
	p := artifactService(t, core, dataRoot)
	for name, expected := range m.UIFiles {
		if name != "index.html" && !strings.HasPrefix(name, "assets/") {
			continue
		}
		path := "/" + name
		if name == "index.html" {
			path = "/"
		}
		r, e := http.Get(p.address + path)
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(r.Body)
		r.Body.Close()
		if e != nil || r.StatusCode != 200 {
			t.Fatal("embedded UI missing", name, e)
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != expected {
			t.Fatal("stale embedded UI", name)
		}
	}
	if _, e = os.Stat(filepath.Join(root, "state.sqlite")); !os.IsNotExist(e) {
		t.Fatal("runtime data in install directory")
	}
	z, e := zip.OpenReader(root + ".zip")
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	if len(z.File) != len(m.Files)+1 {
		t.Fatal("archive inventory differs", len(z.File), len(m.Files))
	}
	seen := map[string]bool{}
	for _, entry := range z.File {
		if seen[entry.Name] {
			t.Fatal("duplicate archive member")
		}
		seen[entry.Name] = true
		expected, ok := m.Files[entry.Name]
		if entry.Name == "manifest.json" {
			expected = checksum(t, filepath.Join(root, "manifest.json"))
			ok = true
		}
		if !ok {
			t.Fatal("unexpected archive member")
		}
		reader, e := entry.Open()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(reader)
		reader.Close()
		if e != nil {
			t.Fatal("archive decompression/CRC failed", e)
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != expected {
			t.Fatal("archive bytes differ", entry.Name)
		}
		if runtime.GOOS != "windows" && (entry.Name == launcher.CoreName() || strings.Contains(entry.Name, "roooooom-launcher")) && entry.Mode()&0111 == 0 {
			t.Fatal("archive lost executable permission")
		}
	}
}
func TestUpgradePreservesPrivateData(t *testing.T) {
	bundle, _ := artifact(t)
	core := filepath.Join(bundle, launcher.CoreName())
	dataRoot := filepath.Join(t.TempDir(), "保留 資料")
	db, e := store.Open(t.Context(), filepath.Join(dataRoot, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	identity := db.Identity()
	_, e = db.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO meta(key,value) VALUES('upgrade-canary','preserve-me')")
		return model.Receipt{}, e
	})
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	if e = privatefs.WriteAtomic(dataRoot, "credentials/private-canary.json", []byte("PRIVATE_UPGRADE_CANARY")); e != nil {
		t.Fatal(e)
	}
	profileRoot, _ := client.ProfileRoot(dataRoot, "offline-profile")
	profile := client.Profile{Root: profileRoot, Handle: "offline-profile", ProjectID: "synthetic", Mode: "offline", Provisional: true}
	if e = client.SaveProfile(profile); e != nil {
		t.Fatal(e)
	}
	cli := client.New(profile, nil)
	_, e = cli.Do(t.Context(), model.Command{Kind: "task.create", Payload: model.JSON(model.TaskCreate{ProjectID: "synthetic", Goal: "pending before upgrade", Scope: []string{"fixture"}, NonScope: []string{}, AuthorityRefs: []string{}, Risk: "normal", AcceptancePolicy: model.AcceptancePolicy{Mode: "integrator", Conditions: []string{"fixture"}}})})
	if e == nil {
		t.Fatal("provisional fact became authoritative")
	}
	before, e := cli.Pending(t.Context())
	cli.Close()
	if e != nil || len(before) != 1 {
		t.Fatal(e)
	}
	first := artifactService(t, core, dataRoot)
	controller, e := launcher.Connect(dataRoot, bundle)
	if e != nil {
		t.Fatal(e)
	}
	conflict := exec.Command(core, "serve", "--data-dir", dataRoot, "--listen", "127.0.0.1:0")
	conflict.Env = onlySystemEnv(t)
	hideProcess(conflict)
	if e = conflict.Run(); e == nil {
		t.Fatal("concurrent writer admitted")
	}
	if e = controller.Stop(t.Context()); e != nil {
		t.Fatal(e)
	}
	<-first.done
	backup := exec.Command(core, "admin", "backup", "--data-dir", dataRoot)
	backup.Env = onlySystemEnv(t)
	hideProcess(backup)
	if out, e := backup.CombinedOutput(); e != nil {
		t.Fatal("upgrade backup failed", e, string(out))
	}
	// A new installation never overwrites the retained old bundle or data root.
	next := filepath.Join(t.TempDir(), "新套件")
	if e = os.MkdirAll(next, 0700); e != nil {
		t.Fatal(e)
	}
	nextCore := filepath.Join(next, launcher.CoreName())
	if e = os.WriteFile(nextCore, []byte("deliberately invalid candidate binary"), 0700); e != nil {
		t.Fatal(e)
	}
	bad := exec.Command(nextCore, "serve", "--data-dir", dataRoot)
	bad.Env = onlySystemEnv(t)
	hideProcess(bad)
	if e = bad.Run(); e == nil {
		t.Fatal("bad upgrade admitted")
	}
	rollback := artifactService(t, core, dataRoot)
	controller, e = launcher.Connect(dataRoot, bundle)
	if e != nil {
		t.Fatal(e)
	}
	if e = controller.Stop(t.Context()); e != nil {
		t.Fatal(e)
	}
	<-rollback.done
	raw, e := os.ReadFile(core)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(nextCore, raw, 0700); e != nil {
		t.Fatal(e)
	}
	launcherName := "roooooom-launcher" + filepath.Ext(core)
	raw, e = os.ReadFile(filepath.Join(bundle, launcherName))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(next, launcherName), raw, 0700); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if c, e := launcher.Connect(dataRoot, next); e == nil {
			c.Stop(context.Background())
		}
	})
	startEntry := func(path string) {
		entry := exec.Command(path, "--headless", "--no-open", "--data-dir", dataRoot, "--listen", "127.0.0.1:0")
		entry.Env = onlySystemEnv(t)
		hideProcess(entry)
		if out, e := entry.CombinedOutput(); e != nil {
			t.Fatal("packaged launcher upgrade/rollback failed", e, string(out))
		}
	}
	startEntry(filepath.Join(next, launcherName))
	controller, e = launcher.Connect(dataRoot, next)
	if e != nil {
		t.Fatal(e)
	}
	if e = controller.Stop(t.Context()); e != nil {
		t.Fatal(e)
	}
	startEntry(filepath.Join(bundle, launcherName))
	controller, e = launcher.Connect(dataRoot, bundle)
	if e != nil {
		t.Fatal(e)
	}
	if e = controller.Stop(t.Context()); e != nil {
		t.Fatal(e)
	}
	db, e = store.Open(context.Background(), filepath.Join(dataRoot, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if db.Identity() != identity {
		t.Fatal("upgrade silently replaced dataset")
	}
	var canary string
	if e = db.QueryRow(t.Context(), "SELECT value FROM meta WHERE key='upgrade-canary'").Scan(&canary); e != nil || canary != "preserve-me" {
		t.Fatal("old data lost", e)
	}
	raw, e = os.ReadFile(filepath.Join(dataRoot, "credentials/private-canary.json"))
	if e != nil || string(raw) != "PRIVATE_UPGRADE_CANARY" {
		t.Fatal("credential moved or lost", e)
	}
	cli = client.New(profile, nil)
	after, e := cli.Pending(t.Context())
	cli.Close()
	if e != nil || len(after) != 1 || after[0].LocalID != before[0].LocalID || after[0].Authoritative {
		t.Fatal("outbox lost or replayed", e)
	}
	if _, e = os.Stat(filepath.Join(next, "credentials")); !os.IsNotExist(e) {
		t.Fatal("credentials moved into installation")
	}
}
func TestUnverifiedPlatformNotAdvertised(t *testing.T) {
	raw, e := os.ReadFile("../packaging/platforms.json")
	if e != nil {
		t.Fatal(e)
	}
	var matrix struct {
		Platforms []struct {
			Target                 string `json:"target"`
			Supported              bool   `json:"supported"`
			NativeEvidenceVerified bool   `json:"nativeEvidenceVerified"`
			CleanProfileVerified   bool   `json:"cleanProfileVerified"`
		} `json:"platforms"`
	}
	if e = json.Unmarshal(raw, &matrix); e != nil || len(matrix.Platforms) == 0 {
		t.Fatal(e)
	}
	for _, p := range matrix.Platforms {
		if p.Supported && (!p.NativeEvidenceVerified || !p.CleanProfileVerified) {
			t.Fatal("unverified platform advertised", p.Target)
		}
	}
}

func TestBundleDataRootCannotBeInsideInstallation(t *testing.T) {
	install := t.TempDir()
	core := filepath.Join(install, launcher.CoreName())
	raw, e := os.ReadFile(binary(t))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(core, raw, 0700); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, core, "serve", "--data-dir", install, "--listen", "127.0.0.1:0")
	cmd.Env = onlySystemEnv(t)
	hideProcess(cmd)
	if e = cmd.Run(); e == nil {
		t.Fatal("installation admitted runtime data")
	}
	if _, e = os.Stat(filepath.Join(install, "state.sqlite")); !os.IsNotExist(e) {
		t.Fatal("runtime database was written inside installation")
	}
}
