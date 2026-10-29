//go:build windows

package runtime

import (
	"path/filepath"
	"testing"
)

func TestDefaultRootUsesLocalAppData(t *testing.T) {
	local, roaming := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("APPDATA", roaming)
	got, err := DefaultRoot()
	if err != nil || got != filepath.Join(local, "Roooooom") {
		t.Fatal("default root must use LOCALAPPDATA")
	}
}
