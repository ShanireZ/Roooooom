package backup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"roooooom.local/roooooom/internal/store"
)

func Restore(ctx context.Context, root, snapshot string) error {
	root, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	manifest, e := Validate(ctx, snapshot)
	if e != nil {
		return e
	}
	rt, e := appruntime.Open(root)
	if e != nil {
		return e
	}
	defer rt.Close()
	livePath := filepath.Join(root, "state.sqlite")
	if _, e = os.Lstat(livePath); e == nil {
		live, e := store.Open(ctx, livePath)
		if e != nil {
			return e
		}
		if live.Identity().ServiceID != manifest.ServiceID {
			live.Close()
			return model.Conflict("snapshot-identity-mismatch")
		}
		preserved := filepath.Join(root, "recovery", string((model.RandomIDs{}).NewID()))
		_, e = Create(ctx, live, preserved)
		if e == nil {
			e = live.Checkpoint(ctx)
		}
		closeErr := live.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	staging := filepath.Join(root, "restore-staging", string((model.RandomIDs{}).NewID()))
	if e = privatefs.CopyAtomic(staging, "state.sqlite", filepath.Join(snapshot, "state.sqlite")); e != nil {
		return e
	}
	staged, e := store.Open(ctx, filepath.Join(staging, "state.sqlite"))
	if e != nil {
		return e
	}
	_, e = staged.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		for _, key := range []string{"generation", "cursor_key"} {
			if _, e := tx.Exec("UPDATE meta SET value=? WHERE key=?", string((model.RandomIDs{}).NewID()), key); e != nil {
				return model.Receipt{}, e
			}
		}
		for _, query := range []string{
			"UPDATE tokens SET revoked=1",
			"UPDATE sessions SET status='ended',version=version+1,body=json_set(body,'$.restored',json('true'),'$.revalidationRequired',json('true'))",
			"UPDATE bindings SET version=version+1,body=json_set(body,'$.enabled',json('false'),'$.needsRebind',json('true'))",
			"UPDATE external_jobs SET status=CASE WHEN status='sending' THEN 'uncertain' WHEN status IN('queued','retry') THEN 'paused-restore' ELSE status END",
		} {
			if _, e := tx.Exec(query); e != nil {
				return model.Receipt{}, e
			}
		}
		rows, e := tx.Query("SELECT body FROM tasks")
		if e != nil {
			return model.Receipt{}, e
		}
		tasks := []model.Task{}
		for rows.Next() {
			var raw string
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return model.Receipt{}, e
			}
			var task model.Task
			if e = json.Unmarshal([]byte(raw), &task); e != nil {
				rows.Close()
				return model.Receipt{}, e
			}
			tasks = append(tasks, task)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return model.Receipt{}, e
		}
		for _, task := range tasks {
			task.Version++
			task.IntegratorEpoch++
			task.CurrentCandidateID = ""
			if task.Phase == "done" {
				task.Phase = "review"
			}
			if _, e = tx.Exec("UPDATE tasks SET version=?,integrator_epoch=?,phase=?,body=? WHERE id=?", task.Version, task.IntegratorEpoch, task.Phase, string(model.JSON(task)), task.ID); e != nil {
				return model.Receipt{}, e
			}
		}
		rows, e = tx.Query("SELECT body FROM assignments")
		if e != nil {
			return model.Receipt{}, e
		}
		assignments := []model.Assignment{}
		for rows.Next() {
			var raw string
			if e = rows.Scan(&raw); e != nil {
				rows.Close()
				return model.Receipt{}, e
			}
			var assignment model.Assignment
			if e = json.Unmarshal([]byte(raw), &assignment); e != nil {
				rows.Close()
				return model.Receipt{}, e
			}
			assignments = append(assignments, assignment)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return model.Receipt{}, e
		}
		for _, assignment := range assignments {
			assignment.Version++
			assignment.ClaimEpoch++
			assignment.AcceptedDeliveryID = ""
			if assignment.CurrentDeliveryID != "" && (assignment.State == "accepted" || assignment.State == "submitted") {
				assignment.State = "submitted"
			}
			if _, e = tx.Exec("UPDATE assignments SET version=?,claim_epoch=?,state=?,body=? WHERE id=?", assignment.Version, assignment.ClaimEpoch, assignment.State, string(model.JSON(assignment)), assignment.ID); e != nil {
				return model.Receipt{}, e
			}
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": "dataset.restored", "revalidationRequired": true, "externalReconcileRequired": true}))
		return model.Receipt{EventID: seq}, e
	})
	if e == nil {
		e = staged.Checkpoint(ctx)
	}
	closeErr := staged.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if _, e = inspect(ctx, filepath.Join(staging, "state.sqlite")); e != nil {
		return e
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		path := livePath + suffix
		if info, e := os.Lstat(path); e == nil {
			if !info.Mode().IsRegular() {
				return privatefs.ErrUnsafePath
			}
			if e = os.Remove(path); e != nil {
				return e
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	return privatefs.CopyAtomic(root, "state.sqlite", filepath.Join(staging, "state.sqlite"))
}
