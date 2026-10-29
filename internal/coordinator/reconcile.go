package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type reconcilePayload struct {
	ProjectID         model.ID      `json:"projectId"`
	LocalID           string        `json:"localId"`
	OriginalRequestID string        `json:"originalRequestId"`
	OriginalCommand   model.Command `json:"originalCommand"`
	BaseCursor        string        `json:"baseCursor"`
	LocalOccurredAt   time.Time     `json:"localOccurredAt"`
}
type reconcileResult struct {
	LocalID     string         `json:"localId"`
	State       string         `json:"state"`
	Reason      string         `json:"reason"`
	Receipt     *model.Receipt `json:"receipt,omitempty"`
	CandidateID model.ID       `json:"candidateId,omitempty"`
}

func (s *Service) reconcile(tx *store.Tx, a model.Actor, p reconcilePayload) (model.ID, int64, []byte, error) {
	if a.Kind != "agent" {
		return "", 0, nil, model.Forbidden()
	}
	if p.ProjectID == "" || p.LocalID == "" || len(p.LocalID) > 128 || p.OriginalRequestID == "" || p.OriginalRequestID != p.OriginalCommand.Meta.ClientRequestID || p.LocalOccurredAt.IsZero() {
		return "", 0, nil, model.Invalid("complete offline source references required")
	}
	if p.OriginalCommand.Kind == "offline.reconcile" || p.OriginalCommand.Kind == "offline.resolve" {
		return "", 0, nil, model.Invalid("nested reconciliation is not allowed")
	}
	typed, e := prepare(p.OriginalCommand)
	if e != nil {
		return "", 0, nil, e
	}
	p.OriginalCommand.Payload = model.JSON(typed)
	scope, scopeErr := commandScope(tx, p.OriginalCommand, typed)
	if scopeErr != nil && model.Status(scopeErr) != 404 {
		return "", 0, nil, scopeErr
	}
	if scopeErr == nil && scope != p.ProjectID {
		return "", 0, nil, model.Forbidden()
	}
	result := reconcileResult{LocalID: p.LocalID, State: "candidate", Reason: "explicit review required; no claim, approval or completion applied"}
	identity := s.store.Identity()
	sameDataset := p.OriginalCommand.Meta.ServiceID == identity.ServiceID && p.OriginalCommand.Meta.Generation == identity.Generation
	if p.OriginalCommand.Meta.ServiceID == identity.ServiceID && p.OriginalCommand.Meta.Generation == identity.Generation {
		digest, e := model.CanonicalHash(p.OriginalCommand)
		if e != nil {
			return "", 0, nil, e
		}
		prior, e := tx.FindReceipt(a.ID, "commands", p.OriginalRequestID, digest)
		if e != nil {
			return "", 0, nil, e
		}
		if prior != nil {
			result.State = "committed"
			result.Reason = "recovered original committed receipt"
			result.Receipt = prior
			return prior.ResourceID, prior.Version, model.JSON(result), nil
		}
	} else {
		result.Reason = "service or dataset generation differs; original operation remains a candidate"
	}
	if sameDataset || p.OriginalCommand.Meta.ServiceID == "" {
		if receipt, ok := typed.(*receiptPayload); ok && receipt.State == "fetched" && scopeErr == nil {
			var state string
			var current int64
			if queryErr := tx.QueryRow("SELECT state,version FROM message_receipts WHERE message_id=? AND participant_id=?", p.OriginalCommand.TargetID, a.ParticipantID).Scan(&state, &current); queryErr == nil {
				if state == "stored" {
					id, version, body, ackErr := s.updateReceipt(tx, a, model.Command{TargetID: p.OriginalCommand.TargetID, Meta: model.Mutation{ExpectedVersion: current, SessionID: a.SessionID}}, receiptPayload{State: "fetched"})
					if ackErr != nil {
						return "", 0, nil, ackErr
					}
					if _, e = tx.AppendEvent("receipt:"+string(a.ParticipantID), model.JSON(map[string]any{"kind": "receipt.update", "resourceId": id, "version": version, "receipt": json.RawMessage(body), "imported": true})); e != nil {
						return "", 0, nil, e
					}
				}
				result.State = "recorded-fact"
				result.Reason = "late client fetched confirmation; seen and acceptance are unchanged"
				return p.OriginalCommand.TargetID, current, model.JSON(result), nil
			}
		}
		if message, ok := typed.(*messagePayload); ok && message.RecipientMode == "participants" && scopeErr == nil {
			if _, e = tx.Exec("SAVEPOINT import_fact"); e != nil {
				return "", 0, nil, e
			}
			copy := *message
			copy.LocalOccurredAt = &p.LocalOccurredAt
			id, version, raw, sendErr := s.sendMessage(tx, a, model.Command{Meta: model.Mutation{SessionID: a.SessionID}}, copy)
			if sendErr == nil {
				var record messageRecord
				if e = json.Unmarshal(raw, &record); e != nil {
					return "", 0, nil, e
				}
				record.Imported = true
				record.OfflineSource = model.JSON(map[string]any{"localId": p.LocalID, "originalRequestId": p.OriginalRequestID, "originalSessionId": p.OriginalCommand.Meta.SessionID, "baseVersions": p.OriginalCommand.Meta})
				raw = model.JSON(record)
				if _, e = tx.Exec("UPDATE messages SET body=? WHERE id=?", string(raw), id); e != nil {
					return "", 0, nil, e
				}
				if _, e = tx.AppendEvent("message:"+string(id), model.JSON(map[string]any{"kind": "message.send", "resourceId": id, "version": version, "message": json.RawMessage(raw), "actorKind": "agent", "imported": true})); e != nil {
					return "", 0, nil, e
				}
				if _, e = tx.Exec("RELEASE import_fact"); e != nil {
					return "", 0, nil, e
				}
				result.State = "recorded-fact"
				result.Reason = "late append-only message; no authority transferred"
				return id, version, model.JSON(result), nil
			}
			if _, e = tx.Exec("ROLLBACK TO import_fact"); e != nil {
				return "", 0, nil, e
			}
			if _, e = tx.Exec("RELEASE import_fact"); e != nil {
				return "", 0, nil, e
			}
			if model.Status(sendErr) >= 500 {
				return "", 0, nil, sendErr
			}
		}
	}
	var existing model.ID
	if e = tx.QueryRow("SELECT id FROM offline_candidates WHERE actor_id=? AND local_id=?", a.ID, p.LocalID).Scan(&existing); e == nil {
		result.CandidateID = existing
		return existing, 1, model.JSON(result), nil
	}
	id := s.ids.NewID()
	tasks, e := relatedTasks(tx, p.OriginalCommand, typed)
	if e != nil {
		return "", 0, nil, e
	}
	body := model.JSON(offlineCandidate{ID: id, ProjectID: p.ProjectID, ActorID: a.ID, SessionID: a.SessionID, Source: p, State: "candidate", Version: 1, TaskIDs: tasks, ReceivedAt: s.clock.Now()})
	if _, e = tx.Exec("INSERT INTO offline_candidates VALUES(?,?,?,?,?)", id, p.ProjectID, a.ID, p.LocalID, string(body)); e != nil {
		return "", 0, nil, e
	}
	result.CandidateID = id
	return id, 1, model.JSON(result), nil
}
