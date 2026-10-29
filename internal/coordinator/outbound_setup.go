package coordinator

import (
	"context"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/store"
)

type Options struct{ RepositoryInspector outbound.RepositoryInspector }

func (s *Service) inspectRepository(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	var p struct {
		Repository    string `json:"repository"`
		CredentialRef string `json:"credentialRef"`
	}
	if e := model.Decode(c.Payload, &p); e != nil {
		return model.Receipt{}, e
	}
	if c.TargetID != "" {
		return model.Receipt{}, model.Invalid("inspection has no target")
	}
	if _, e := outbound.RepositoryName(p.Repository); e != nil {
		return model.Receipt{}, e
	}
	hash, e := model.CanonicalHash(c)
	if e != nil {
		return model.Receipt{}, e
	}
	var prior *model.Receipt
	_, e = s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var err error
		prior, err = tx.FindReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash)
		return model.Receipt{}, err
	})
	if e != nil {
		return model.Receipt{}, e
	}
	if prior != nil {
		return *prior, nil
	}
	inspector := s.repositoryInspector
	if inspector == nil {
		inspector = outbound.NewGitHub(outbound.ResolveCredential(filepath.Dir(s.store.Path)))
	}
	// Network reads run outside the SQLite writer transaction.
	inspected, e := inspector.Inspect(ctx, p.Repository, p.CredentialRef)
	if e != nil {
		return model.Receipt{}, e
	}
	return s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		prior, e := tx.FindReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash)
		if e != nil {
			return model.Receipt{}, e
		}
		if prior != nil {
			return *prior, nil
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "repository": inspected.Repository, "repositoryId": inspected.RepositoryID}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, Version: 1, EventID: seq, Result: model.JSON(inspected)}
		return r, tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
	})
}
