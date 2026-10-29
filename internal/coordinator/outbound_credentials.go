package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"strings"
	"sync"
)

var credentialMu sync.Mutex

func (s *Service) storeOutboundCredential(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	var p struct {
		Kind       string `json:"kind"`
		Token      string `json:"token"`
		Secret     string `json:"secret,omitempty"`
		WebhookURL string `json:"webhookUrl,omitempty"`
	}
	if e := model.Decode(c.Payload, &p); e != nil {
		return model.Receipt{}, e
	}
	if p.WebhookURL != "" {
		if p.Kind != "dingtalk" || p.Token != "" {
			return model.Receipt{}, model.Invalid("one credential input required")
		}
		token, e := outbound.ParseWebhook(p.WebhookURL)
		if e != nil {
			return model.Receipt{}, e
		}
		p.Token = token
		p.WebhookURL = ""
	}
	if c.TargetID != "" || (p.Kind != "github" && p.Kind != "dingtalk") || p.Token == "" || len(p.Token) > 512 || strings.ContainsAny(p.Token, "\r\n\t ") || len(p.Secret) > 512 || p.Kind == "dingtalk" && p.Secret == "" || p.Kind == "github" && p.Secret != "" {
		return model.Receipt{}, model.Invalid("supported private integration credential required")
	}
	hash, e := model.CanonicalHash(c)
	if e != nil {
		return model.Receipt{}, e
	}
	credentialMu.Lock()
	defer credentialMu.Unlock()
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
	sum := sha256.Sum256([]byte(s.store.Identity().Generation + ":" + string(a.ID) + ":" + hash))
	ref := hex.EncodeToString(sum[:])
	// Secret material exists only in the protected product-owned file. An
	// interrupted audit leaves an unreferenced private file, never a binding.
	if e = privatefs.WriteAtomic(filepath.Dir(s.store.Path), filepath.Join("credentials", ref+".json"), model.JSON(p)); e != nil {
		return model.Receipt{}, model.Invalid("private credential could not be stored")
	}
	return s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": ref, "integrationKind": p.Kind}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: model.ID(ref), Version: 1, EventID: seq, Result: model.JSON(map[string]any{"credentialRef": ref, "kind": p.Kind})}
		e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
		return r, e
	})
}
func (s *Service) rotateOutboundCredential(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	var p struct {
		CredentialRef string `json:"credentialRef"`
		Reason        string `json:"reason"`
	}
	if e := model.Decode(c.Payload, &p); e != nil {
		return model.Receipt{}, e
	}
	if p.Reason == "" || len(p.Reason) > 8192 {
		return model.Receipt{}, model.Invalid("rotation reason required")
	}
	hash, e := model.CanonicalHash(c)
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
		var raw string
		var version int64
		if e = tx.QueryRow("SELECT body,version FROM bindings WHERE id=?", c.TargetID).Scan(&raw, &version); e != nil {
			return model.Receipt{}, model.NotFound()
		}
		var b outbound.Binding
		if e = json.Unmarshal([]byte(raw), &b); e != nil {
			return model.Receipt{}, e
		}
		if c.Meta.ExpectedVersion != version || b.NeedsRebind || !b.Enabled {
			return model.Receipt{}, model.Conflict("stale-or-disabled-binding")
		}
		channel, e := outbound.ValidatePrivateCredential(filepath.Dir(s.store.Path), b.Kind, p.CredentialRef)
		if e != nil {
			return model.Receipt{}, e
		}
		if b.Kind == "dingtalk" && channel != b.ChannelID {
			return model.Receipt{}, model.Conflict("new-bot-requires-new-binding")
		}
		b.Version = version + 1
		b.CredentialRef = p.CredentialRef
		if _, e = tx.Exec("UPDATE bindings SET version=?,body=? WHERE id=?", b.Version, string(model.JSON(b)), b.ID); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": b.ID, "reason": p.Reason}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: b.ID, Version: b.Version, EventID: seq, Result: model.JSON(b)}
		e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
		return r, e
	})
}
