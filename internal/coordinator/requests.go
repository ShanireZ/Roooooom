package coordinator

import (
	"encoding/json"
	"strings"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/store"
)

type requestTransition struct {
	Reason                  string   `json:"reason,omitempty"`
	ToSessionID             model.ID `json:"toSessionId,omitempty"`
	ExpectedSourceSessionID model.ID `json:"expectedSourceSessionId,omitempty"`
}
type requestConversion struct {
	Task model.TaskCreate `json:"task"`
}

func loadRequest(tx rowReader, id model.ID) (model.WorkRequest, error) {
	var r model.WorkRequest
	var raw string
	if e := tx.QueryRow("SELECT body FROM work_requests WHERE id=?", id).Scan(&raw); e != nil {
		return r, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &r)
	return r, e
}
func saveRequest(tx *store.Tx, r *model.WorkRequest) error {
	old := r.Version
	r.Version++
	result, e := tx.Exec("UPDATE work_requests SET version=?,state=?,preparer_participant_id=?,preparer_session_id=?,preparation_epoch=?,task_id=?,body=? WHERE id=? AND version=?", r.Version, r.State, nullable(r.PreparerParticipantID), nullable(r.PreparerSessionID), r.PreparationEpoch, nullable(r.TaskID), string(model.JSON(r)), r.ID, old)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return model.Conflict("stale-request")
	}
	return nil
}
func preparationOwner(a model.Actor, r model.WorkRequest, m model.Mutation) error {
	if a.Kind != "agent" || a.ParticipantID != r.PreparerParticipantID || a.SessionID != r.PreparerSessionID || m.ClaimEpoch != r.PreparationEpoch || r.State != "preparing" {
		return model.Conflict("stale-preparation")
	}
	return nil
}
func targetPreparationSession(tx rowReader, id, project model.ID) (model.ID, error) {
	var participant model.ID
	e := tx.QueryRow("SELECT s.participant_id FROM sessions s JOIN tokens t ON t.session_id=s.id AND t.kind='agent' AND t.revoked=0 AND t.project_id=? JOIN participant_projects p ON p.participant_id=s.participant_id AND p.project_id=t.project_id WHERE s.id=? AND s.status='active' LIMIT 1", project, id).Scan(&participant)
	if e != nil {
		return "", model.Invalid("active project session required")
	}
	return participant, nil
}
func (s *Service) createRequest(tx *store.Tx, a model.Actor, p model.RequestCreate) (model.ID, int64, []byte, error) {
	if a.Kind != "human" {
		return "", 0, nil, model.Forbidden()
	}
	p.Text = strings.TrimSpace(p.Text)
	if p.Constraints == nil {
		p.Constraints = []string{}
	}
	if p.Text == "" || len(p.Text) > 8192 || !validStrings(p.Constraints, true) || p.CollaboratorLimit != nil && (*p.CollaboratorLimit < 0 || *p.CollaboratorLimit > 64) {
		return "", 0, nil, model.Invalid("request text and valid optional requirements required")
	}
	var count int
	if e := tx.QueryRow("SELECT count(*) FROM projects WHERE id=?", p.ProjectID).Scan(&count); e != nil {
		return "", 0, nil, e
	}
	if count != 1 {
		return "", 0, nil, model.NotFound()
	}
	if p.DesignatedIntegratorID != "" && !participantInProject(tx, p.DesignatedIntegratorID, p.ProjectID) {
		return "", 0, nil, model.Invalid("designated integrator not in project")
	}
	if p.AcceptancePolicy != nil {
		if e := validatePolicy(tx, p.ProjectID, *p.AcceptancePolicy); e != nil {
			return "", 0, nil, e
		}
	}
	r := model.WorkRequest{RequestCreate: p, ID: s.ids.NewID(), Version: 1, State: "waiting", CreatedBy: a.ID, ActorKind: "human", CreatedAt: s.clock.Now()}
	_, e := tx.Exec("INSERT INTO work_requests(id,project_id,version,state,preparation_epoch,body) VALUES(?,?,1,'waiting',0,?)", r.ID, r.ProjectID, string(model.JSON(r)))
	return r.ID, r.Version, model.JSON(r), e
}
func (s *Service) transitionRequest(tx *store.Tx, a model.Actor, c model.Command, p requestTransition) (model.ID, int64, []byte, error) {
	r, e := loadRequest(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if (c.Kind == "request.cancel" || c.Kind == "request.assign") && a.Kind != "human" {
		return "", 0, nil, model.Forbidden()
	}
	if e = version(c.Meta.ExpectedVersion, r.Version); e != nil {
		return "", 0, nil, e
	}
	if r.State == "converted" || r.State == "cancelled" {
		return "", 0, nil, model.Conflict("request-closed")
	}
	switch c.Kind {
	case "request.claim":
		if a.Kind != "agent" {
			return "", 0, nil, model.Forbidden()
		}
		if r.State != "waiting" {
			return "", 0, nil, model.Conflict("preparation-occupied")
		}
		r.PreparerParticipantID = a.ParticipantID
		r.PreparerSessionID = a.SessionID
		r.PreparationEpoch++
		r.State = "preparing"
	case "request.release", "request.handoff":
		if e = preparationOwner(a, r, c.Meta); e != nil {
			return "", 0, nil, e
		}
		if strings.TrimSpace(p.Reason) == "" || len(p.Reason) > 8192 {
			return "", 0, nil, model.Invalid("preparation reason required")
		}
		if c.Kind == "request.release" {
			r.PreparerParticipantID = ""
			r.PreparerSessionID = ""
			r.OfferedSessionID = ""
			r.State = "waiting"
			r.PreparationEpoch++
		} else {
			if p.ToSessionID == a.SessionID {
				return "", 0, nil, model.Invalid("different target required")
			}
			if _, e = targetPreparationSession(tx, p.ToSessionID, r.ProjectID); e != nil {
				return "", 0, nil, e
			}
			r.OfferedSessionID = p.ToSessionID
		}
	case "request.accept":
		if a.Kind != "agent" || r.OfferedSessionID != a.SessionID || r.State != "preparing" || c.Meta.ClaimEpoch != r.PreparationEpoch {
			return "", 0, nil, model.Conflict("preparation-offer-changed")
		}
		r.PreparerParticipantID = a.ParticipantID
		r.PreparerSessionID = a.SessionID
		r.PreparationEpoch++
		r.OfferedSessionID = ""
	case "request.resume":
		if a.Kind != "agent" || r.State != "preparing" || a.ParticipantID != r.PreparerParticipantID || p.ExpectedSourceSessionID != r.PreparerSessionID || c.Meta.ClaimEpoch != r.PreparationEpoch {
			return "", 0, nil, model.Conflict("stale-preparation")
		}
		var status string
		if e = tx.QueryRow("SELECT status FROM sessions WHERE id=?", r.PreparerSessionID).Scan(&status); e != nil {
			return "", 0, nil, e
		}
		if status != "ended" {
			return "", 0, nil, model.Conflict("source-session-still-active")
		}
		r.PreparerSessionID = a.SessionID
		r.PreparationEpoch++
		r.OfferedSessionID = ""
	case "request.assign":
		if strings.TrimSpace(p.Reason) == "" || len(p.Reason) > 8192 {
			return "", 0, nil, model.Invalid("takeover reason required")
		}
		participant, err := targetPreparationSession(tx, p.ToSessionID, r.ProjectID)
		if err != nil {
			return "", 0, nil, err
		}
		r.PreparerParticipantID = participant
		r.PreparerSessionID = p.ToSessionID
		r.PreparationEpoch++
		r.OfferedSessionID = ""
		r.State = "preparing"
	case "request.cancel":
		if strings.TrimSpace(p.Reason) == "" || len(p.Reason) > 8192 {
			return "", 0, nil, model.Invalid("cancellation reason required")
		}
		r.State = "cancelled"
		r.OfferedSessionID = ""
		r.PreparationEpoch++
		if e = s.cancelRequestQuestions(tx, a, r); e != nil {
			return "", 0, nil, e
		}
	default:
		return "", 0, nil, model.Invalid("unknown request transition")
	}
	r.LastReason = p.Reason
	if e = saveRequest(tx, &r); e != nil {
		return "", 0, nil, e
	}
	return r.ID, r.Version, model.JSON(r), nil
}
func (s *Service) convertRequest(tx *store.Tx, a model.Actor, c model.Command, p requestConversion) (model.ID, int64, []byte, error) {
	r, e := loadRequest(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, r.Version); e != nil {
		return "", 0, nil, e
	}
	if e = preparationOwner(a, r, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if r.TaskID != "" || p.Task.ProjectID != r.ProjectID {
		return "", 0, nil, model.Forbidden()
	}
	decisions, e := requestDecisionVersions(tx, r, c.Meta)
	if e != nil {
		return "", 0, nil, e
	}
	if r.CollaboratorLimit != nil && p.Task.CollaboratorLimit != *r.CollaboratorLimit {
		return "", 0, nil, model.Forbidden()
	}
	if r.DesignatedIntegratorID != "" && p.Task.DesignatedIntegratorID != r.DesignatedIntegratorID {
		return "", 0, nil, model.Forbidden()
	}
	if r.AcceptancePolicy != nil {
		old := r.AcceptancePolicy
		next := p.Task.AcceptancePolicy
		if old.Mode != next.Mode || old.AcceptorID != next.AcceptorID || string(model.JSON(old.Conditions)) != string(model.JSON(next.Conditions)) {
			return "", 0, nil, model.Forbidden()
		}
		p.Task.AcceptancePolicy.OwnerProtected = false
	}
	id, _, _, e := s.createTask(tx, a, p.Task)
	if e != nil {
		return "", 0, nil, e
	}
	task, e := loadTask(tx, id)
	if e != nil {
		return "", 0, nil, e
	}
	task.SourceRequestID = r.ID
	task.SourceRequestText = r.Text
	task.SourceConstraints = append([]string{}, r.Constraints...)
	task.DecisionVersions = decisions
	if r.CollaboratorLimit != nil {
		task.CollaboratorLimitSource = "owner"
	}
	if r.AcceptancePolicy != nil {
		s.initializePolicy(&task, model.Actor{Kind: "human"})
		task.AcceptanceConstraints.ModeSource = "owner"
		task.AcceptanceConstraints.AcceptorSource = "owner"
		task.AcceptancePolicy.OwnerProtected = true
	}
	if _, e = tx.Exec("UPDATE tasks SET body=? WHERE id=?", string(model.JSON(task)), task.ID); e != nil {
		return "", 0, nil, e
	}
	r.TaskID = task.ID
	r.ConvertedSpecVersion = task.SpecVersion
	r.State = "converted"
	r.OfferedSessionID = ""
	if e = saveRequest(tx, &r); e != nil {
		return "", 0, nil, e
	}
	seq, e := tx.AppendEvent(string(r.ProjectID), model.JSON(map[string]any{"kind": "task.create", "resourceId": task.ID, "version": task.Version, "actorId": a.ID, "actorKind": a.Kind, "sessionId": a.SessionID, "receivedAt": s.clock.Now(), "sourceRequestId": r.ID}))
	if e != nil {
		return "", 0, nil, e
	}
	identity := s.store.Identity()
	if e = outbound.Enqueue(tx, identity.ServiceID, identity.Generation, "task.create", task.ID, r.ProjectID, seq); e != nil {
		return "", 0, nil, e
	}
	if e = outbound.EnqueueNotifications(tx, identity.ServiceID, identity.Generation, task.ID); e != nil {
		return "", 0, nil, e
	}
	return r.ID, r.Version, model.JSON(map[string]any{"request": r, "task": task}), nil
}
