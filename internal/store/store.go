package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	goruntime "runtime"
	"sync"
)

//go:embed migrations/*.sql
var migrations embed.FS

const ApplicationID = 0x524F4F4D
const SchemaVersion = 2

type Store struct {
	wakeMu                    sync.Mutex
	wake                      chan struct{}
	cursorKey                 string
	writer, reader            *sql.DB
	lock                      *flock.Flock
	beforeCommit, afterCommit func()
	identity                  Identity
	Path                      string
}
type Identity struct{ ServiceID, Generation string }
type Tx struct {
	tx  *sql.Tx
	ctx context.Context
}
type Reader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func Open(ctx context.Context, path string) (result *Store, err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = privatefs.EnsureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if info, e := os.Lstat(path); e == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return nil, privatefs.ErrUnsafePath
	}
	s := &Store{Path: path, lock: flock.New(path + ".writer.lock"), wake: make(chan struct{})}
	locked, e := s.lock.TryLock()
	if e != nil || !locked {
		s.lock.Close()
		return nil, errors.New("database writer already running")
	}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	basePath := filepath.ToSlash(path)
	if goruntime.GOOS == "windows" {
		basePath = "/" + basePath
	}
	u := url.URL{Scheme: "file", Path: basePath}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	s.writer, err = sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	s.writer.SetMaxOpenConns(1)
	s.writer.SetMaxIdleConns(1)
	if err = s.writer.PingContext(ctx); err != nil {
		return nil, err
	}
	var application, version, tables int
	if err = s.writer.QueryRowContext(ctx, "PRAGMA application_id").Scan(&application); err != nil {
		return nil, err
	}
	if err = s.writer.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if err = s.writer.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		return nil, err
	}
	if (application != ApplicationID && (application != 0 || tables != 0)) || version > SchemaVersion {
		return nil, errors.New("unrelated or newer database")
	}
	var mode string
	if err = s.writer.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return nil, err
	}
	if mode != "wal" {
		return nil, errors.New("WAL unavailable")
	}
	// A schema migration owns the only writer before the server accepts work.
	// SQLite's table rebuild requires FK enforcement off outside the transaction;
	// explicitly check all references before committing and restore enforcement.
	if version < SchemaVersion {
		if _, err = s.writer.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return nil, err
		}
	}
	defer func() {
		if s.writer != nil {
			_, _ = s.writer.ExecContext(context.Background(), "PRAGMA foreign_keys=ON")
		}
	}()
	tx, e := s.writer.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	for next := version + 1; next <= SchemaVersion; next++ {
		file := map[int]string{1: "migrations/0001_core.sql", 2: "migrations/0002_requests.sql"}[next]
		schema, readErr := migrations.ReadFile(file)
		if readErr != nil {
			return nil, readErr
		}
		if _, err = tx.ExecContext(ctx, string(schema)); err != nil {
			return nil, err
		}
	}
	ids := model.RandomIDs{}
	for _, key := range []string{"service_id", "generation", "cursor_key"} {
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO meta(key,value) VALUES(?,?)", key, string(ids.NewID())); err != nil {
			return nil, err
		}
	}
	violations, checkErr := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if checkErr != nil {
		return nil, checkErr
	}
	broken := violations.Next()
	checkErr = violations.Err()
	violations.Close()
	if checkErr != nil {
		return nil, checkErr
	}
	if broken {
		return nil, errors.New("migration foreign-key integrity failed")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if _, err = s.writer.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return nil, err
	}
	if err = s.writer.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='service_id'").Scan(&s.identity.ServiceID); err != nil {
		return nil, err
	}
	if err = s.writer.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='generation'").Scan(&s.identity.Generation); err != nil {
		return nil, err
	}
	if err = s.writer.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='cursor_key'").Scan(&s.cursorKey); err != nil {
		return nil, err
	}
	q.Set("mode", "ro")
	q.Del("_txlock")
	q.Add("_pragma", "query_only(1)")
	u.RawQuery = q.Encode()
	s.reader, err = sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	s.reader.SetMaxOpenConns(4)
	s.reader.SetMaxIdleConns(4)
	if err = s.reader.PingContext(ctx); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Store) Identity() Identity { return s.identity }
func (s *Store) OpenReaders() int   { return s.reader.Stats().InUse }
func (s *Store) Read(ctx context.Context, fn func(Reader) error) error {
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(tx)
}
func (s *Store) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.reader.QueryRowContext(ctx, query, args...)
}
func (s *Store) Close() error {
	var result error
	if s.reader != nil {
		result = errors.Join(result, s.reader.Close())
	}
	if s.writer != nil {
		result = errors.Join(result, s.writer.Close())
	}
	if s.lock != nil {
		result = errors.Join(result, s.lock.Close())
	}
	return result
}
