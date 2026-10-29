package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"testing"
)

// Recreating the parent table without preserving rows or references would destroy a real schema-1 history.
func TestSchemaOneMigratesWithoutHistoryLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.sqlite")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	schema, e := migrations.ReadFile("migrations/0001_core.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(string(schema)); e != nil {
		t.Fatal(e)
	}
	fixture := `INSERT INTO meta VALUES('service_id','same-service'),('generation','same-generation'),('cursor_key','same-key');
 INSERT INTO projects VALUES('p',1,'{}');
 INSERT INTO tasks VALUES('t','p',1,1,'ready',0,NULL,NULL,0,'{}');
 INSERT INTO decision_requests VALUES('q','p','t',2,'choice','resolved','{"id":"q","history":"preserve"}');
 INSERT INTO decisions VALUES('d','q',1,'{"answer":"保留歷史"}');`
	if _, e = db.Exec(fixture); e != nil {
		t.Fatal(e)
	}
	db.Close()
	s, e := Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	var version int
	var body string
	if e = s.QueryRow(context.Background(), "PRAGMA user_version").Scan(&version); e != nil || version != 2 {
		t.Fatalf("schema not upgraded: %d %v", version, e)
	}
	if e = s.QueryRow(context.Background(), "SELECT body FROM decisions WHERE id='d'").Scan(&body); e != nil || body != `{"answer":"保留歷史"}` {
		t.Fatal(body, e)
	}
	if s.Identity().ServiceID != "same-service" || s.Identity().Generation != "same-generation" {
		t.Fatal(s.Identity())
	}
	s.Close()
	s, e = Open(context.Background(), path)
	if e != nil {
		t.Fatal("reopen", e)
	}
	defer s.Close()
	var check string
	if e = s.QueryRow(context.Background(), "PRAGMA integrity_check").Scan(&check); e != nil || check != "ok" {
		t.Fatal(check, e)
	}
	_, e = s.Write(context.Background(), func(tx *Tx) (model.Receipt, error) {
		_, err := tx.Exec("INSERT INTO decisions VALUES('bad','missing',1,'{}')")
		return model.Receipt{}, err
	})
	if e == nil {
		t.Fatal("foreign keys left disabled after migration")
	}
}
