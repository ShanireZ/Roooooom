package privatefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateRootRejectsEscape(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../outside", filepath.Join(root, "absolute"), ".", ""} {
		if err := WriteAtomic(root, name, []byte("secret")); err == nil {
			t.Fatalf("accepted escaping name %q", name)
		}
	}
	if err := WriteAtomic(root, "nested/state.json", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(root, "nested/state.json", []byte("second")); err != nil {
		t.Fatal(err)
	}
	value, err := os.ReadFile(filepath.Join(root, "nested/state.json"))
	if err != nil || string(value) != "second" {
		t.Fatalf("replacement %q: %v", value, err)
	}
}

func TestPrivateRootRejectsSymlink(t *testing.T) {
	root := existingLinkFixture(t)
	// Read-only validation of a real junction: never risk writing into an OS location.
	if err := CheckRoot(root); err == nil {
		t.Fatal("accepted linked root")
	}
	if err := CheckRoot(filepath.Join(root, "roooooom-not-created", "nested")); err == nil {
		t.Fatal("accepted linked ancestor")
	}
}
