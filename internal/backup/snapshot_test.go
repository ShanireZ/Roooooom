package backup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"roooooom.local/roooooom/internal/store"
	"testing"
)

func TestInterruptedSnapshotChild(t *testing.T) {
	if os.Getenv("ROOOOOOM_SNAPSHOT_CHILD") != "1" {
		return
	}
	st, e := store.Open(context.Background(), filepath.Join(os.Getenv("ROOOOOOM_TEST_ROOT"), "live", "state.sqlite"))
	if e != nil {
		os.Exit(41)
	}
	_, e = create(context.Background(), st, filepath.Join(os.Getenv("ROOOOOOM_TEST_ROOT"), "snapshot"), func() { os.Exit(42) })
	if e != nil {
		os.Exit(43)
	}
	os.Exit(44)
}
func TestInterruptedAndCorruptSnapshots(t *testing.T) {
	root := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestInterruptedSnapshotChild$")
	command.Env = append(os.Environ(), "ROOOOOOM_SNAPSHOT_CHILD=1", "ROOOOOOM_TEST_ROOT="+root)
	e := command.Run()
	if code, ok := e.(*exec.ExitError); !ok || code.ExitCode() != 42 {
		t.Fatalf("child did not reach publication boundary: %v", e)
	}
	target := filepath.Join(root, "snapshot")
	if _, e := Validate(t.Context(), target); e == nil {
		t.Fatal("incomplete snapshot accepted")
	}
	st, e := store.Open(t.Context(), filepath.Join(root, "live", "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	if _, e := Create(t.Context(), st, target); e == nil {
		t.Fatal("interrupted snapshot silently replaced")
	}
	complete := filepath.Join(root, "complete")
	if _, e := Create(t.Context(), st, complete); e != nil {
		t.Fatal(e)
	}
	file, e := os.OpenFile(filepath.Join(complete, "state.sqlite"), os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	_, e = file.WriteAt([]byte("broken database"), 0)
	file.Close()
	if e != nil {
		t.Fatal(e)
	}
	if _, e := Validate(t.Context(), complete); e == nil {
		t.Fatal("corrupt snapshot accepted")
	}
	other, e := store.Open(t.Context(), filepath.Join(root, "other", "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	fresh := filepath.Join(root, "fresh")
	if _, e := Create(t.Context(), st, fresh); e != nil {
		t.Fatal(e)
	}
	if _, e := Create(t.Context(), other, fresh); e == nil {
		t.Fatal("unrelated snapshot accepted as current backup")
	}
	otherID := other.Identity()
	otherRoot := filepath.Dir(other.Path)
	other.Close()
	if e := Restore(t.Context(), otherRoot, fresh); e == nil {
		t.Fatal("restore overwrote a different service root")
	}
	reopened, e := store.Open(t.Context(), filepath.Join(otherRoot, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if reopened.Identity() != otherID {
		t.Fatal("rejected restore changed existing identity")
	}
}
