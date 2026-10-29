package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"github.com/gofrs/flock"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"runtime"
	"time"
)

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	ServiceID     string    `json:"serviceId"`
	Generation    string    `json:"datasetGeneration"`
	Hash          string    `json:"sha256"`
	CreatedAt     time.Time `json:"createdAt"`
	EventSeq      int64     `json:"eventSeq"`
}

func readonly(path string) (*sql.DB, error) {
	if e := privatefs.CheckRoot(filepath.Dir(path)); e != nil {
		return nil, e
	}
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, privatefs.ErrUnsafePath
	}
	file := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		file = "/" + file
	}
	u := url.URL{Scheme: "file", Path: file}
	q := url.Values{"mode": {"ro"}, "_pragma": {"query_only(1)", "foreign_keys(1)", "busy_timeout(5000)"}}
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	return db, nil
}
func digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func inspect(ctx context.Context, path string) (Manifest, error) {
	db, e := readonly(path)
	if e != nil {
		return Manifest{}, e
	}
	defer db.Close()
	var app int
	var check string
	m := Manifest{}
	if e = db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&app); e != nil {
		return m, e
	}
	if app != store.ApplicationID {
		return m, model.Invalid("unrelated snapshot")
	}
	if e = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&m.SchemaVersion); e != nil {
		return m, e
	}
	if m.SchemaVersion < 1 || m.SchemaVersion > store.SchemaVersion {
		return m, model.Invalid("unsupported snapshot schema")
	}
	if e = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); e != nil {
		return m, e
	}
	if check != "ok" {
		return m, model.Invalid("snapshot integrity failed")
	}
	if e = db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='service_id'").Scan(&m.ServiceID); e != nil {
		return m, e
	}
	if e = db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='generation'").Scan(&m.Generation); e != nil {
		return m, e
	}
	if e = db.QueryRowContext(ctx, "SELECT COALESCE(MAX(seq),0) FROM events").Scan(&m.EventSeq); e != nil {
		return m, e
	}
	return m, nil
}
func Validate(ctx context.Context, target string) (Manifest, error) {
	raw, e := privatefs.ReadFile(target, "manifest.json")
	if e != nil {
		return Manifest{}, e
	}
	var manifest Manifest
	if json.Unmarshal(raw, &manifest) != nil {
		return manifest, model.Invalid("invalid snapshot manifest")
	}
	observed, e := inspect(ctx, filepath.Join(target, "state.sqlite"))
	if e != nil {
		return manifest, e
	}
	hash, e := digest(filepath.Join(target, "state.sqlite"))
	if e != nil {
		return manifest, e
	}
	if manifest.SchemaVersion != observed.SchemaVersion || manifest.ServiceID != observed.ServiceID || manifest.Generation != observed.Generation || manifest.EventSeq != observed.EventSeq || manifest.Hash != hash || manifest.CreatedAt.IsZero() {
		return manifest, model.Invalid("snapshot manifest mismatch")
	}
	return manifest, nil
}
func Create(ctx context.Context, st *store.Store, target string) (Manifest, error) {
	return create(ctx, st, target, nil)
}
func create(ctx context.Context, st *store.Store, target string, beforeManifest func()) (Manifest, error) {
	target, e := filepath.Abs(target)
	if e != nil {
		return Manifest{}, e
	}
	if e = privatefs.EnsureDir(target); e != nil {
		return Manifest{}, e
	}
	lock := flock.New(filepath.Join(target, "snapshot.lock"))
	locked, e := lock.TryLock()
	if e != nil || !locked {
		lock.Close()
		return Manifest{}, model.Conflict("snapshot-busy")
	}
	defer lock.Close()
	if _, e = os.Lstat(filepath.Join(target, "manifest.json")); e == nil {
		prior, e := Validate(ctx, target)
		identity := st.Identity()
		if e == nil && (prior.ServiceID != identity.ServiceID || prior.Generation != identity.Generation) {
			return Manifest{}, model.Conflict("snapshot-identity-mismatch")
		}
		return prior, e
	}
	if _, e = os.Lstat(filepath.Join(target, "state.sqlite")); e == nil {
		return Manifest{}, model.Conflict("incomplete-snapshot")
	}
	temporary := ".snapshot-" + string((model.RandomIDs{}).NewID()) + ".sqlite"
	if e = privatefs.WriteAtomic(target, temporary, nil); e != nil {
		return Manifest{}, e
	}
	defer os.Remove(filepath.Join(target, temporary))
	if e = st.VacuumInto(ctx, filepath.Join(target, temporary)); e != nil {
		return Manifest{}, e
	}
	manifest, e := inspect(ctx, filepath.Join(target, temporary))
	if e != nil {
		return manifest, e
	}
	manifest.Hash, e = digest(filepath.Join(target, temporary))
	if e != nil {
		return manifest, e
	}
	manifest.CreatedAt = time.Now().UTC()
	f, e := os.OpenFile(filepath.Join(target, temporary), os.O_RDWR, 0600)
	if e != nil {
		return manifest, e
	}
	e = f.Sync()
	f.Close()
	if e != nil {
		return manifest, e
	}
	if e = privatefs.Publish(target, temporary, "state.sqlite"); e != nil {
		return manifest, e
	}
	if beforeManifest != nil {
		beforeManifest()
	}
	if e = privatefs.WriteAtomic(target, "manifest.json", model.JSON(manifest)); e != nil {
		return manifest, e
	}
	return manifest, nil
}
