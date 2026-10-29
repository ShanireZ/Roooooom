package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type messagePayload struct {
	ProjectID       model.ID   `json:"projectId"`
	TaskID          model.ID   `json:"taskId,omitempty"`
	AssignmentID    model.ID   `json:"assignmentId,omitempty"`
	ThreadID        string     `json:"threadId"`
	Purpose         string     `json:"purpose"`
	Priority        string     `json:"priority"`
	Body            string     `json:"body"`
	RecipientMode   string     `json:"recipientMode"`
	RecipientIDs    []model.ID `json:"recipientIds"`
	ActionRequired  bool       `json:"actionRequired"`
	LocalOccurredAt *time.Time `json:"localOccurredAt,omitempty"`
}
type messageRecord struct {
	messagePayload
	ID            model.ID        `json:"id"`
	ActorID       model.ID        `json:"actorId"`
	ActorKind     string          `json:"actorKind"`
	SessionID     model.ID        `json:"sessionId,omitempty"`
	ReceivedAt    time.Time       `json:"receivedAt"`
	Imported      bool            `json:"imported,omitempty"`
	OfflineSource json.RawMessage `json:"offlineSource,omitempty"`
}

func loadMessage(tx rowReader, id model.ID) (messageRecord, error) {
	var message messageRecord
	var raw string
	if e := tx.QueryRow("SELECT body FROM messages WHERE id=?", id).Scan(&raw); e != nil {
		return message, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &message)
	return message, e
}
func (s *Service) sendMessage(tx *store.Tx, a model.Actor, c model.Command, p messagePayload) (model.ID, int64, []byte, error) {
	if p.ProjectID == "" || p.ThreadID == "" || len(p.ThreadID) > 128 || p.Body == "" || len(p.Body) > 16*1024 || p.RecipientIDs == nil || len(p.RecipientIDs) > 200 {
		return "", 0, nil, model.Invalid("bounded message and recipient contract required")
	}
	switch p.Purpose {
	case "question", "proposal", "contract-change", "handoff", "delivery", "decision-notice", "status", "discussion":
	default:
		return "", 0, nil, model.Invalid("invalid message purpose")
	}
	if p.Priority != "normal" && p.Priority != "urgent" {
		return "", 0, nil, model.Invalid("invalid priority")
	}
	var task model.Task
	if p.TaskID != "" {
		var e error
		task, e = loadTask(tx, p.TaskID)
		if e != nil {
			return "", 0, nil, e
		}
		if task.ProjectID != p.ProjectID {
			return "", 0, nil, model.Invalid("task outside project")
		}
	}
	recipients := map[model.ID]bool{}
	switch p.RecipientMode {
	case "integrator":
		if task.ID == "" || task.IntegratorID == "" {
			return "", 0, nil, model.Conflict("integrator-required")
		}
		if len(p.RecipientIDs) != 0 {
			return "", 0, nil, model.Invalid("role recipients are resolved by server")
		}
		recipients[task.IntegratorID] = true
	case "assignment":
		assignment, e := loadAssignment(tx, p.AssignmentID)
		if e != nil {
			return "", 0, nil, e
		}
		if assignment.TaskID != task.ID || assignment.ClaimParticipantID == "" {
			return "", 0, nil, model.Invalid("assignment recipient unavailable")
		}
		if len(p.RecipientIDs) != 0 {
			return "", 0, nil, model.Invalid("role recipients are resolved by server")
		}
		recipients[assignment.ClaimParticipantID] = true
	case "participants":
		if len(p.RecipientIDs) == 0 {
			return "", 0, nil, model.Invalid("recipients required")
		}
		for _, id := range p.RecipientIDs {
			if !participantInProject(tx, id, p.ProjectID) {
				return "", 0, nil, model.Invalid("recipient outside project")
			}
			if recipients[id] {
				return "", 0, nil, model.Invalid("duplicate recipient")
			}
			recipients[id] = true
		}
	case "project":
		if a.Kind != "human" {
			if task.ID == "" {
				return "", 0, nil, model.Forbidden()
			}
			if e := manager(a, task, c.Meta); e != nil {
				return "", 0, nil, e
			}
		}
		if len(p.RecipientIDs) != 0 {
			return "", 0, nil, model.Invalid("broadcast recipients are resolved by server")
		}
		rows, e := tx.Query("SELECT participant_id FROM participant_projects WHERE project_id=?", p.ProjectID)
		if e != nil {
			return "", 0, nil, e
		}
		for rows.Next() {
			var id model.ID
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return "", 0, nil, e
			}
			recipients[id] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", 0, nil, e
		}
	default:
		return "", 0, nil, model.Invalid("invalid recipient mode")
	}
	p.RecipientIDs = []model.ID{}
	for id := range recipients {
		p.RecipientIDs = append(p.RecipientIDs, id)
	}
	sortIDs(p.RecipientIDs)
	message := messageRecord{messagePayload: p, ID: s.ids.NewID(), ActorID: a.ID, ActorKind: a.Kind, SessionID: a.SessionID, ReceivedAt: s.clock.Now()}
	if _, e := tx.Exec("INSERT INTO messages VALUES(?,?,?,?,?)", message.ID, p.ProjectID, nullable(p.TaskID), p.ThreadID, string(model.JSON(message))); e != nil {
		return "", 0, nil, e
	}
	for _, id := range p.RecipientIDs {
		if _, e := tx.Exec("INSERT INTO message_receipts VALUES(?,?,'stored',1,'{}')", message.ID, id); e != nil {
			return "", 0, nil, e
		}
	}
	return message.ID, 1, model.JSON(message), nil
}
