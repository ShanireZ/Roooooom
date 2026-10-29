package coordinator

import (
	"context"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

// Preferences applies owner-only presentation settings. They grant no authority.
func (s *Service) Preferences(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
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
	if c.Kind != "owner.language" {
		return model.Receipt{}, model.Invalid("unknown preference command")
	}
	var p struct {
		Language string `json:"language"`
	}
	if e := model.Decode(c.Payload, &p); e != nil {
		return model.Receipt{}, e
	}
	switch p.Language {
	case "", "en", "zh-CN", "zh-TW", "ja":
	default:
		return model.Receipt{}, model.Invalid("language must be en, zh-CN, zh-TW, ja or empty")
	}
	c.Payload = model.JSON(p)
	hash, e := model.CanonicalHash(c)
	if e != nil {
		return model.Receipt{}, e
	}
	return s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		prior, e := tx.FindReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash)
		if e != nil || prior != nil {
			if prior != nil {
				return *prior, nil
			}
			return model.Receipt{}, e
		}
		if e = tx.SetOwnerLanguage(p.Language); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": "owner"}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: "owner", Version: 1, EventID: seq, Result: model.JSON(map[string]string{"language": p.Language})}
		return r, tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
	})
}

// OwnerLanguage is the owner's chosen interface language, or "" when unset.
func (s *Service) OwnerLanguage(ctx context.Context) (string, error) {
	return s.store.OwnerLanguage(ctx)
}
