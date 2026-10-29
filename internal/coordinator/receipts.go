package coordinator

import (
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

type receiptPayload struct {
	State        string   `json:"state"`
	Note         string   `json:"note,omitempty"`
	EvidenceRefs []string `json:"evidenceRefs,omitempty"`
}

func (s *Service) updateReceipt(tx *store.Tx, a model.Actor, c model.Command, p receiptPayload) (model.ID, int64, []byte, error) {
	if a.Kind != "agent" {
		return "", 0, nil, model.Forbidden()
	}
	message, e := loadMessage(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	var state string
	var current int64
	if e = tx.QueryRow("SELECT state,version FROM message_receipts WHERE message_id=? AND participant_id=?", message.ID, a.ParticipantID).Scan(&state, &current); e != nil {
		return "", 0, nil, model.Forbidden()
	}
	if e = version(c.Meta.ExpectedVersion, current); e != nil {
		return "", 0, nil, e
	}
	if len(p.Note) > 8192 || len(p.EvidenceRefs) > 200 {
		return "", 0, nil, model.Invalid("receipt metadata too large")
	}
	allowed := false
	switch p.State {
	case "fetched":
		allowed = state == "stored" || state == "fetched"
	case "seen":
		allowed = state == "fetched" || state == "seen"
	case "accepted", "declined":
		allowed = message.ActionRequired && (state == "seen" || state == p.State) && p.Note != ""
	case "resolved":
		allowed = (state == "accepted" || state == "resolved") && p.Note != "" && validStrings(p.EvidenceRefs, false)
	}
	if !allowed {
		return "", 0, nil, model.Conflict("invalid-receipt-transition")
	}
	result := map[string]any{"messageId": message.ID, "projectId": message.ProjectID, "participantId": a.ParticipantID, "state": p.State, "version": current + 1, "note": p.Note, "evidenceRefs": p.EvidenceRefs, "receivedAt": s.clock.Now(), "actorKind": "agent"}
	if _, e = tx.Exec("UPDATE message_receipts SET state=?,version=version+1,body=? WHERE message_id=? AND participant_id=? AND version=?", p.State, string(model.JSON(result)), message.ID, a.ParticipantID, current); e != nil {
		return "", 0, nil, e
	}
	return message.ID, current + 1, model.JSON(result), nil
}
