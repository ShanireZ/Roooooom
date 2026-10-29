package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/backup"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/identity"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
)

func recoveryFixture(t *testing.T) (*testkit.Fixture, model.ID, model.Actor, model.ID) {
	t.Helper()
	f := testkit.New(t)
	project, _ := f.Project()
	agent := f.Agent(project, "agent")
	task := f.Must(agent, "task.create", "", testkit.TaskPayload(project, 0), f.Meta(agent))
	return f, project, agent, task.ResourceID
}
func addRecoveryMessage(t *testing.T, f *testkit.Fixture, p model.ID, a model.Actor, body string) {
	t.Helper()
	f.Must(a, "message.send", "", map[string]any{"projectId": p, "threadId": "recovery", "purpose": "discussion", "priority": "normal", "body": body, "recipientMode": "participants", "recipientIds": []model.ID{a.ParticipantID}, "actionRequired": false}, f.Meta(a))
}
func TestBackupIncludesUncheckpointedWAL(t *testing.T) {
	f, p, a, _ := recoveryFixture(t)
	for range 12 {
		addRecoveryMessage(t, f, p, a, "committed WAL witness")
	}
	info, e := os.Stat(f.Store.Path + "-wal")
	if e != nil || info.Size() == 0 {
		t.Fatal("fixture has no uncheckpointed WAL", e)
	}
	target := filepath.Join(t.TempDir(), "snapshot")
	manifest, e := backup.Create(t.Context(), f.Store, target)
	if e != nil {
		t.Fatal(e)
	}
	if manifest.ServiceID != f.Store.Identity().ServiceID || manifest.SchemaVersion != 2 || len(manifest.Hash) != 64 {
		t.Fatal(manifest)
	}
	db, e := sql.Open("sqlite", filepath.Join(target, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var count int
	if e = db.QueryRow("SELECT count(*) FROM messages").Scan(&count); e != nil || count != 12 {
		t.Fatal("WAL commits missing from snapshot", count, e)
	}
}
func TestRestoreGenerationAndOutbox(t *testing.T) {
	f, p, a, task := recoveryFixture(t)
	addRecoveryMessage(t, f, p, a, "in snapshot")
	cursor, e := f.Core.Sync(t.Context(), f.Owner(), "")
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.Store.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
		if _, e := tx.Exec("INSERT INTO bindings VALUES('b',?,1,'github',?)", p, `{"enabled":true,"credentialRef":"private-not-restored"}`); e != nil {
			return model.Receipt{}, e
		}
		_, e := tx.Exec("INSERT INTO external_jobs(id,binding_id,task_id,version,kind,status,dedupe_key,retry_at,body) VALUES('j','b',?,1,'issue-create','sending','unique',NULL,'{}')", task)
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if _, e = backup.Create(t.Context(), f.Store, snapshot); e != nil {
		t.Fatal(e)
	}
	root := filepath.Dir(f.Store.Path)
	before := f.Store.Identity()
	if e = backup.Restore(t.Context(), root, snapshot); e == nil {
		t.Fatal("restore overwrote a live writer")
	}
	addRecoveryMessage(t, f, p, a, "preserve this current database")
	secretFile, e := os.ReadFile(filepath.Join(root, "credentials", "owner.json"))
	if e != nil {
		t.Fatal(e)
	}
	var old struct{ Token string }
	_ = json.Unmarshal(secretFile, &old)
	if e = f.Store.Close(); e != nil {
		t.Fatal(e)
	}
	if e = backup.Restore(t.Context(), root, snapshot); e != nil {
		t.Fatal(e)
	}
	restored, e := store.Open(t.Context(), filepath.Join(root, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	if restored.Identity().ServiceID != before.ServiceID || restored.Identity().Generation == before.Generation {
		t.Fatal("restore did not fence the dataset")
	}
	ids := identity.New(restored, nil)
	if _, e = ids.Authenticate(t.Context(), old.Token); model.Status(e) != 401 {
		t.Fatal("old owner cookie survived restore", e)
	}
	core := coordinator.New(restored, nil, nil)
	if _, e = core.Sync(t.Context(), f.Owner(), cursor.Cursor); model.Status(e) != 409 {
		t.Fatal("old cursor was accepted", e)
	}
	var state string
	if e = restored.QueryRow(t.Context(), "SELECT status FROM external_jobs WHERE id='j'").Scan(&state); e != nil || state != "uncertain" {
		t.Fatal("unknown external effect was replayable", state, e)
	}
	var enabled int
	if e = restored.QueryRow(t.Context(), "SELECT json_extract(body,'$.enabled') FROM bindings WHERE id='b'").Scan(&enabled); e != nil || enabled != 0 {
		t.Fatal("binding enabled without revalidation", e)
	}
	preserved, e := os.ReadDir(filepath.Join(root, "recovery"))
	if e != nil || len(preserved) == 0 {
		t.Fatal("original data was not preserved", e)
	}
}
func TestRestoredGenerationRejectsOldMutation(t *testing.T) {
	f, _, _, _ := recoveryFixture(t)
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	old := f.Meta(f.Owner())
	old.ClientRequestID = "old-generation"
	if _, e := backup.Create(t.Context(), f.Store, snapshot); e != nil {
		t.Fatal(e)
	}
	root := filepath.Dir(f.Store.Path)
	f.Store.Close()
	if e := backup.Restore(t.Context(), root, snapshot); e != nil {
		t.Fatal(e)
	}
	restored, e := store.Open(t.Context(), filepath.Join(root, "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	core := coordinator.New(restored, nil, nil)
	if _, e = core.Apply(t.Context(), f.Owner(), model.Command{Kind: "project.create", Meta: old, Payload: model.JSON(coordinator.ProjectPayload{Name: "must not exist", Repositories: []coordinator.RepositoryPayload{}})}); model.Status(e) != 409 {
		t.Fatal("old generation mutation accepted", e)
	}
	var count int
	if e = restored.QueryRow(t.Context(), "SELECT count(*) FROM projects").Scan(&count); e != nil || count != 1 {
		t.Fatal("old mutation changed rows", count, e)
	}
}
func TestStorageFullAndPrivateExport(t *testing.T) {
	f, p, a, _ := recoveryFixture(t)
	root := filepath.Dir(f.Store.Path)
	raw, e := os.ReadFile(filepath.Join(root, "credentials", "owner.json"))
	if e != nil {
		t.Fatal(e)
	}
	var secret struct{ Token, CSRF string }
	_ = json.Unmarshal(raw, &secret)
	// max_page_count injects SQLITE_FULL without consuming the user's actual disk.
	_, e = f.Store.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("PRAGMA max_page_count=100")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 30; i++ {
		_, e = f.Apply(a, "message.send", "", map[string]any{"projectId": p, "threadId": "capacity", "purpose": "discussion", "priority": "normal", "body": strings.Repeat("x", 16*1024), "recipientMode": "participants", "recipientIds": []model.ID{a.ParticipantID}, "actionRequired": false}, f.Meta(a))
		if e != nil {
			break
		}
	}
	if model.Status(e) != 507 {
		t.Fatal("full storage not distinguished", e)
	}
	target := filepath.Join(t.TempDir(), "export")
	if e = backup.Export(context.Background(), f.Store, target); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(filepath.Join(target, "data.json"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(data, []byte(secret.Token)) || bytes.Contains(data, []byte(secret.CSRF)) || bytes.Contains(data, []byte(`"tokens"`)) || bytes.Contains(data, []byte(`"cursor_key"`)) {
		t.Fatal("credentials exported")
	}
}
