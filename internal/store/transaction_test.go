package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func mutate(s *Store, hash string) (model.Receipt, error) {
	return s.Write(context.Background(), func(tx *Tx) (model.Receipt, error) {
		old, err := tx.FindReceipt("actor", "scope", "request", hash)
		if err != nil {
			return model.Receipt{}, err
		}
		if old != nil {
			return *old, nil
		}
		id, err := tx.AppendEvent("scope", []byte(`{"kind":"test"}`))
		if err != nil {
			return model.Receipt{}, err
		}
		r := model.Receipt{RequestID: "request", ResourceID: "object", Version: 1, EventID: id, Result: json.RawMessage(`{"ok":true}`)}
		return r, tx.SaveReceipt("actor", "scope", "request", hash, r)
	})
}
func TestReceiptReplayAndPayloadConflict(t *testing.T) {
	s := openTest(t)
	a, err := mutate(s, "hash")
	if err != nil {
		t.Fatal(err)
	}
	b, err := mutate(s, "hash")
	if err != nil || a.EventID != b.EventID {
		t.Fatal("replayed a new effect", err)
	}
	_, err = mutate(s, "different")
	if model.Status(err) != 409 {
		t.Fatal("key collision not rejected", err)
	}
	var count int
	if err = s.reader.QueryRow("SELECT count(*) FROM events").Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate event", count, err)
	}
}
func TestRollbackCallbackDoesNotPublish(t *testing.T) {
	s := openTest(t)
	_, err := s.Write(context.Background(), func(tx *Tx) (model.Receipt, error) {
		_, e := tx.AppendEvent("scope", []byte("{}"))
		if e != nil {
			return model.Receipt{}, e
		}
		return model.Receipt{}, errors.New("abort")
	})
	if err == nil {
		t.Fatal("callback error lost")
	}
	var n int
	if err = s.reader.QueryRow("SELECT count(*) FROM events").Scan(&n); err != nil || n != 0 {
		t.Fatal("uncommitted event visible")
	}
}
func TestSQLitePragmasEveryConnection(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	var conns []*sql.Conn
	for i := 0; i < 4; i++ {
		c, err := s.reader.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()
	for _, c := range conns {
		for pragma, want := range map[string]int{"foreign_keys": 1, "busy_timeout": 5000, "synchronous": 2, "query_only": 1} {
			var got int
			if err := c.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil || got != want {
				t.Fatal("pragma", pragma, got, err)
			}
		}
		var mode string
		if err := c.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
			t.Fatal(mode, err)
		}
	}
	var version string
	_ = s.writer.QueryRow("SELECT sqlite_version()").Scan(&version)
	t.Log("SQLite engine", version)
}
func TestStoreReadOnlyConnection(t *testing.T) {
	s := openTest(t)
	if _, err := s.reader.Exec("CREATE TABLE illicit(id TEXT)"); err == nil {
		t.Fatal("reader can mutate")
	}
}
