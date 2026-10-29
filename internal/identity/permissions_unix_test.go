//go:build !windows

package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateCredentialFileACL(t *testing.T) {
	s, st, _ := fixture(t)
	activate(t, s)
	info, e := os.Stat(filepath.Join(filepath.Dir(st.Path), "credentials", "owner.json"))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("credential permissions", e)
	}
}
