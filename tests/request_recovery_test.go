package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/backup"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestRequestBackupAndReadableExport(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	r := f.Must(f.Owner(), "request.create", "", map[string]any{"projectId": p, "text": "保存這個未接手需求"}, f.Meta(f.Owner()))
	target := filepath.Join(t.TempDir(), "snapshot")
	manifest, e := backup.Create(t.Context(), f.Store, target)
	if e != nil {
		t.Fatal(e)
	}
	if manifest.SchemaVersion != 2 {
		t.Fatal("incorrect snapshot schema", manifest.SchemaVersion)
	}
	clone, e := store.Open(t.Context(), filepath.Join(target, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer clone.Close()
	var body string
	if e = clone.QueryRow(t.Context(), "SELECT body FROM work_requests WHERE id=?", r.ResourceID).Scan(&body); e != nil {
		t.Fatal("intake missing from snapshot", e)
	}
	var request struct{ Text string }
	if e = json.Unmarshal([]byte(body), &request); e != nil || request.Text != "保存這個未接手需求" {
		t.Fatal(body, e)
	}
	export := filepath.Join(t.TempDir(), "export")
	if e = backup.Export(t.Context(), f.Store, export); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(export, "data.json"))
	if e != nil {
		t.Fatal(e)
	}
	var data struct {
		SchemaVersion int                          `json:"schemaVersion"`
		Tables        map[string][]json.RawMessage `json:"tables"`
	}
	if e = json.Unmarshal(raw, &data); e != nil || data.SchemaVersion != 2 || len(data.Tables["work_requests"]) != 1 {
		t.Fatal("request export missing", e)
	}
}
