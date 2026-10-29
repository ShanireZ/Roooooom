package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"roooooom.local/roooooom/internal/control"
	"roooooom.local/roooooom/internal/launcher"
	"testing"
)

func TestFinalLauncherSideBySideUpgradeAndRollback(t *testing.T) {
	old, next := t.TempDir(), t.TempDir()
	root := filepath.Join(t.TempDir(), "同一 私有資料")
	raw, e := os.ReadFile(binary(t))
	if e != nil {
		t.Fatal(e)
	}
	for _, bundle := range []string{old, next} {
		if e = os.WriteFile(filepath.Join(bundle, launcher.CoreName()), raw, 0700); e != nil {
			t.Fatal(e)
		}
	}
	name := "roooooom-launcher"
	if filepath.Ext(launcher.CoreName()) == ".exe" {
		name += ".exe"
	}
	oldEntry := filepath.Join(old, name)
	cmd := exec.Command("go", "build", "-o", oldEntry, "./cmd/roooooom-launcher")
	cmd.Dir = ".."
	hideProcess(cmd)
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	raw, e = os.ReadFile(oldEntry)
	if e != nil {
		t.Fatal(e)
	}
	newEntry := filepath.Join(next, name)
	if e = os.WriteFile(newEntry, raw, 0700); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		for _, bundle := range []string{old, next} {
			if c, e := launcher.Connect(root, bundle); e == nil {
				c.Stop(context.Background())
			}
		}
	})
	run := func(entry string) error {
		cmd := exec.Command(entry, "--headless", "--no-open", "--data-dir", root, "--listen", "127.0.0.1:0")
		hideProcess(cmd)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Log(string(out))
		}
		return e
	}
	if e = run(oldEntry); e != nil {
		t.Fatal(e)
	}
	first, e := control.Read(root)
	if e != nil {
		t.Fatal(e)
	}
	if e = run(newEntry); e == nil {
		t.Fatal("new bundle took over a live old installation")
	}
	c, e := launcher.Connect(root, old)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Stop(t.Context()); e != nil {
		t.Fatal(e)
	}
	if e = run(newEntry); e != nil {
		t.Fatal("new launcher cannot start stopped historical installation", e)
	}
	second, e := control.Read(root)
	if e != nil || second.ServiceID != first.ServiceID || second.Generation != first.Generation {
		t.Fatal("upgrade changed identity", e)
	}
	c, e = launcher.Connect(root, next)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Stop(t.Context()); e != nil {
		t.Fatal(e)
	}
	if e = run(oldEntry); e != nil {
		t.Fatal("retained launcher cannot roll back stopped installation", e)
	}
}
