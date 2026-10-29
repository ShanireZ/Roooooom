package runtime

import (
	"errors"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/privatefs"
	goruntime "runtime"
	"strings"
)

var ErrAlreadyRunning = errors.New("service already running for this data directory")

func InstallationContainsDataRoot(install, data string) bool {
	rel, e := filepath.Rel(filepath.Clean(install), filepath.Clean(data))
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

type Runtime struct {
	Root, ServiceID, Generation string
	lock                        *flock.Flock
}

func Open(root string) (result *Runtime, err error) {
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = privatefs.EnsureDir(root); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(root, "writer.lock"))
	locked, err := lock.TryLock()
	if err != nil {
		lock.Close()
		return nil, err
	}
	if !locked {
		lock.Close()
		return nil, ErrAlreadyRunning
	}
	defer func() {
		if err != nil {
			lock.Close()
		}
	}()
	return &Runtime{Root: root, lock: lock}, nil
}
func (r *Runtime) Close() error { return r.lock.Close() }
func DefaultRoot() (string, error) {
	if goruntime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" || !filepath.IsAbs(base) {
			return "", errors.New("LOCALAPPDATA unavailable")
		}
		return filepath.Join(base, "Roooooom"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Roooooom"), nil
}
