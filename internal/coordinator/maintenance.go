package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"roooooom.local/roooooom/internal/backup"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"sync"
)

// Maintenance serializes file publication and its durable audit receipt. A file
// left by a failed receipt commit is private and never a successful operation.
var maintenanceMu sync.Mutex

func (s *Service) Maintenance(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	if a.Kind != "human" {
		return model.Receipt{}, model.Forbidden()
	}
	id := s.store.Identity()
	if c.Meta.ServiceID != id.ServiceID || c.Meta.Generation != id.Generation {
		return model.Receipt{}, model.Conflict("needs-reconcile")
	}
	if c.Meta.ClientRequestID == "" || len(c.Meta.ClientRequestID) > 128 || c.TargetID != "" {
		return model.Receipt{}, model.Invalid("request ID required; no target accepted")
	}
	var p struct{}
	if e := model.Decode(c.Payload, &p); e != nil {
		return model.Receipt{}, e
	}
	if c.Kind != "backup.create" && c.Kind != "backup.export" {
		return model.Receipt{}, model.Invalid("unknown maintenance command")
	}
	c.Payload = model.JSON(p)
	hash, e := model.CanonicalHash(c)
	if e != nil {
		return model.Receipt{}, e
	}
	maintenanceMu.Lock()
	defer maintenanceMu.Unlock()
	var prior *model.Receipt
	_, e = s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var e error
		prior, e = tx.FindReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash)
		return model.Receipt{}, e
	})
	if e != nil {
		return model.Receipt{}, e
	}
	if prior != nil {
		return *prior, nil
	}
	sum := sha256.Sum256([]byte(id.Generation + "|" + string(a.ID) + "|" + c.Meta.ClientRequestID))
	resource := model.ID(hex.EncodeToString(sum[:]))
	folder := "backups"
	if c.Kind == "backup.export" {
		folder = "exports"
	}
	target := filepath.Join(filepath.Dir(s.store.Path), folder, string(resource))
	result := map[string]any{"id": resource, "path": target}
	if c.Kind == "backup.create" {
		manifest, err := backup.Create(ctx, s.store, target)
		if err != nil {
			return model.Receipt{}, err
		}
		result["manifest"] = manifest
	} else {
		if e = backup.Export(ctx, s.store, target); e != nil {
			return model.Receipt{}, e
		}
	}
	return s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": resource}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: resource, Version: 1, EventID: seq, Result: model.JSON(result)}
		e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
		return r, e
	})
}
