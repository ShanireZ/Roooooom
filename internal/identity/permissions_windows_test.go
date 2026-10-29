//go:build windows

package identity

import (
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
	"unsafe"
)

func assertPrivateCredential(t *testing.T, path string) {
	t.Helper()
	sd, e := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if e != nil {
		t.Fatal(e)
	}
	control, _, e := sd.Control()
	if e != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("credential DACL not protected")
	}
	acl, _, e := sd.DACL()
	if e != nil || acl == nil || acl.AceCount != 1 {
		t.Fatal("credential grants access beyond owner")
	}
	sid, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		t.Fatal(e)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if e = windows.GetAce(acl, 0, &ace); e != nil {
		t.Fatal(e)
	}
	const fileAllAccess = 0x001f01ff // WinNT FILE_ALL_ACCESS.
	if !(*windows.SID)(unsafe.Pointer(&ace.SidStart)).Equals(sid.User.Sid) || ace.Mask&fileAllAccess != fileAllAccess {
		t.Fatal("credential ACE is not the current user full-access grant")
	}
}
func TestPrivateCredentialFileACL(t *testing.T) {
	s, st, _ := fixture(t)
	activate(t, s)
	assertPrivateCredential(t, filepath.Join(filepath.Dir(st.Path), "credentials", "owner.json"))
}
