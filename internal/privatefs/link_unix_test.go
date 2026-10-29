//go:build !windows

package privatefs

import (
	"os"
	"path/filepath"
	"testing"
)

func existingLinkFixture(t *testing.T) string {
	t.Helper()
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "escape")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	return link
}
