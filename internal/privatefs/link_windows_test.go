//go:build windows

package privatefs

import (
	"os"
	"path/filepath"
	"testing"
)

func existingLinkFixture(t *testing.T) string {
	t.Helper()
	for _, path := range []string{filepath.Join(os.Getenv("SystemDrive")+"/", "Users", "All Users"), filepath.Join(os.Getenv("SystemDrive")+"/", "Documents and Settings")} {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return path
		}
	}
	t.Fatal("read-only Windows junction fixture unavailable")
	return ""
}
