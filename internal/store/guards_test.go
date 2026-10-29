package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"roooooom.local/roooooom/internal/model"
)

func TestConditionalUpdateRejectsStaleVersion(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_, err := s.Write(ctx, func(tx *Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects(id,version,body) VALUES('project',1,'{}')")
		return model.Receipt{}, e
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []int64{1, 1} {
		_, err = s.Write(ctx, func(tx *Tx) (model.Receipt, error) {
			version, e := tx.CompareAndSwap("projects", "project", expected, json.RawMessage("{}"))
			return model.Receipt{Version: version}, e
		})
		if expected == 1 && err != nil {
			var version int64
			if model.Status(err) != 409 {
				t.Fatal(err)
			}
			if e := s.QueryRow(ctx, "SELECT version FROM projects WHERE id='project'").Scan(&version); e != nil || version != 2 {
				t.Fatal(version, e)
			}
			return
		}
	}
	t.Fatal("stale update accepted")
}

func TestUnrelatedDatabaseIsNotModified(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TABLE foreign_data(value TEXT); INSERT INTO foreign_data VALUES('preserve')")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, _ := os.ReadFile(path)
	s, err := Open(context.Background(), path)
	if err == nil {
		s.Close()
		t.Fatal("accepted unrelated database")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("modified unrelated database")
	}
}
