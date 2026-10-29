//go:build windows

package privatefs

import (
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
)

func TestPrivateWindowsACL(t *testing.T) {
	root := t.TempDir()
	if err := WriteAtomic(root, "secret", []byte("fake")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root, filepath.Join(root, "secret")} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		control, _, err := sd.Control()
		if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
			t.Fatal("ACL must be protected")
		}
		acl, _, err := sd.DACL()
		if err != nil || acl == nil || acl.AceCount != 1 {
			t.Fatal("private ACL must have only the current user grant")
		}
	}
}
