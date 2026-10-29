package coordinator

import (
	"context"
	"encoding/json"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/store"
	"strconv"
)

func (s *Service) Outbound(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	if a.Kind != "human" {
		return model.Receipt{}, model.Forbidden()
	}
	i := s.store.Identity()
	if c.Meta.ServiceID != i.ServiceID || c.Meta.Generation != i.Generation {
		return model.Receipt{}, model.Conflict("needs-reconcile")
	}
	if c.Meta.ClientRequestID == "" || len(c.Meta.ClientRequestID) > 128 {
		return model.Receipt{}, model.Invalid("request ID required")
	}
	if c.Kind == "binding.credential" {
		return s.storeOutboundCredential(ctx, a, c)
	}
	if c.Kind == "binding.inspect" {
		return s.inspectRepository(ctx, a, c)
	}
	if c.Kind == "binding.rotate" {
		return s.rotateOutboundCredential(ctx, a, c)
	}
	if c.Kind == "binding.retry" {
		return s.retryOutbound(ctx, a, c)
	}
	var p outbound.Binding
	switch c.Kind {
	case "binding.create":
		if e := model.Decode(c.Payload, &p); e != nil {
			return model.Receipt{}, e
		}
		if e := outbound.Validate(p); e != nil {
			return model.Receipt{}, e
		}
		if c.TargetID != "" || p.ID != "" || p.Version != 0 {
			return model.Receipt{}, model.Invalid("new binding required")
		}
	case "binding.disable":
		var empty struct{}
		if e := model.Decode(c.Payload, &empty); e != nil {
			return model.Receipt{}, e
		}
	default:
		return model.Receipt{}, model.Invalid("unknown binding command")
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
		if c.Kind == "binding.create" {
			var n int
			if e = tx.QueryRow("SELECT count(*) FROM projects WHERE id=?", p.ProjectID).Scan(&n); e != nil {
				return model.Receipt{}, e
			}
			if n != 1 {
				return model.Receipt{}, model.NotFound()
			}
			if e = tx.QueryRow("SELECT count(*) FROM bindings WHERE project_id=? AND kind=? AND json_extract(body,'$.enabled')=1", p.ProjectID, p.Kind).Scan(&n); e != nil {
				return model.Receipt{}, e
			}
			if n != 0 && p.Enabled {
				return model.Receipt{}, model.Conflict("disable-prior-binding-first")
			}
			p.ID = s.ids.NewID()
			p.Version = 1
			p.ChannelID = "github:" + strconv.FormatInt(p.RepositoryID, 10)
			if p.Kind == "dingtalk" {
				credential, err := outbound.ReadBot(outbound.ResolveCredential(filepath.Dir(s.store.Path)), p.CredentialRef)
				if err != nil {
					return model.Receipt{}, model.Invalid("private bot credential required")
				}
				p.ChannelID = outbound.BotChannel(credential.Token)
			}
			_, e = tx.Exec("INSERT INTO bindings(id,project_id,version,kind,body) VALUES(?,?,?,?,?)", p.ID, p.ProjectID, p.Version, p.Kind, string(model.JSON(p)))
		} else {
			var body string
			if e = tx.QueryRow("SELECT body,version FROM bindings WHERE id=?", c.TargetID).Scan(&body, &p.Version); e != nil {
				return model.Receipt{}, model.NotFound()
			}
			version := p.Version
			if e = json.Unmarshal([]byte(body), &p); e != nil {
				return model.Receipt{}, e
			}
			p.Version = version
			if c.Meta.ExpectedVersion != p.Version {
				return model.Receipt{}, model.Conflict("stale")
			}
			p.Enabled = false
			p.Version++
			_, e = tx.Exec("UPDATE bindings SET version=?,body=? WHERE id=?", p.Version, string(model.JSON(p)), p.ID)
		}
		if e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": p.ID, "version": p.Version}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: p.ID, Version: p.Version, EventID: seq, Result: model.JSON(p)}
		e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
		return r, e
	})
}

func (s *Service) retryOutbound(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	var p struct {
		Mode   string `json:"mode"`
		Reason string `json:"reason"`
	}
	if e := model.Decode(c.Payload, &p); e != nil {
		return model.Receipt{}, e
	}
	if (p.Mode != "retry" && p.Mode != "reconcile") || p.Reason == "" || len(p.Reason) > 8192 {
		return model.Receipt{}, model.Invalid("explicit mode and reason required")
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
		var raw, status string
		var current int64
		var enabled, needsRebind bool
		if e = tx.QueryRow("SELECT j.body,j.status,j.version,json_extract(b.body,'$.enabled'),coalesce(json_extract(b.body,'$.needsRebind'),0) FROM external_jobs j JOIN bindings b ON b.id=j.binding_id WHERE j.id=?", c.TargetID).Scan(&raw, &status, &current, &enabled, &needsRebind); e != nil {
			return model.Receipt{}, model.NotFound()
		}
		if current != c.Meta.ExpectedVersion || !enabled || needsRebind {
			return model.Receipt{}, model.Conflict("stale-or-disabled-binding")
		}
		if status != "paused" && status != "uncertain" {
			return model.Receipt{}, model.Conflict("job-not-actionable")
		}
		var job outbound.Job
		if e = json.Unmarshal([]byte(raw), &job); e != nil {
			return model.Receipt{}, e
		}
		if p.Mode == "retry" && (status == "uncertain" || job.OutcomeUncertain && job.Kind != "dingtalk.notify" || job.ErrorCode == "retry-budget-exhausted" && job.Kind == "github.issue") {
			return model.Receipt{}, model.Conflict("reconcile-required")
		}
		job.Status = "uncertain"
		if p.Mode == "retry" {
			job.Status = "retry"
		}
		job.Version = current + 1
		job.Attempts = 0
		job.ErrorCode = ""
		if _, e = tx.Exec("UPDATE external_jobs SET status=?,version=?,retry_at=NULL,body=? WHERE id=?", job.Status, job.Version, string(model.JSON(job)), job.ID); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": job.ID, "mode": p.Mode, "reason": p.Reason}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: job.ID, Version: job.Version, EventID: seq, Result: model.JSON(job)}
		e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, hash, r)
		return r, e
	})
}
