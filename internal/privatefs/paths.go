package privatefs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe private path")

// ReadFile accepts a product-relative regular file, never a project path.
func ReadFile(root, relative string) ([]byte, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return nil, ErrUnsafePath
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, ErrUnsafePath
	}
	path := filepath.Join(root, clean)
	if err := CheckRoot(filepath.Dir(path)); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return nil, ErrUnsafePath
	}
	return os.ReadFile(path)
}

func CheckRoot(root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) == filepath.Dir(filepath.Clean(root)) || strings.HasPrefix(filepath.ToSlash(root), "//") {
		return ErrUnsafePath
	}
	if err := checkVolume(root); err != nil {
		return err
	}
	for p := filepath.Clean(root); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return ErrUnsafePath
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
func EnsureDir(root string) error {
	if err := CheckRoot(root); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := CheckRoot(root); err != nil {
		return err
	}
	return protect(root, true)
}
func WriteAtomic(root, relative string, data []byte) error {
	return WriteStream(root, relative, func(w io.Writer) error { _, e := w.Write(data); return e })
}
func WriteStream(root, relative string, write func(io.Writer) error) error {
	if relative == "" || relative == "." || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return ErrUnsafePath
	}
	clean := filepath.Clean(relative)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return ErrUnsafePath
	}
	if err := EnsureDir(root); err != nil {
		return err
	}
	target := filepath.Join(root, clean)
	parent := filepath.Dir(target)
	if err := EnsureDir(parent); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return ErrUnsafePath
	}
	f, err := os.CreateTemp(parent, ".write-")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = protect(temp, false); err != nil {
		f.Close()
		return err
	}
	if err = write(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return replace(temp, target)
}
func Publish(root, fromRelative, toRelative string) error {
	for _, name := range []string{fromRelative, toRelative} {
		if name == "" || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
			return ErrUnsafePath
		}
		clean := filepath.Clean(name)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return ErrUnsafePath
		}
		if e := CheckRoot(filepath.Dir(filepath.Join(root, clean))); e != nil {
			return e
		}
	}
	from, to := filepath.Join(root, fromRelative), filepath.Join(root, toRelative)
	info, e := os.Lstat(from)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return ErrUnsafePath
	}
	if info, e := os.Lstat(to); e == nil && !info.Mode().IsRegular() {
		return ErrUnsafePath
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if e = protect(from, false); e != nil {
		return e
	}
	return replace(from, to)
}
func CopyAtomic(root, relative, source string) error {
	if e := CheckRoot(filepath.Dir(source)); e != nil {
		return e
	}
	info, e := os.Lstat(source)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return ErrUnsafePath
	}
	f, e := os.Open(source)
	if e != nil {
		return e
	}
	defer f.Close()
	return WriteStream(root, relative, func(w io.Writer) error { _, e := io.Copy(w, f); return e })
}
