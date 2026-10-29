package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type candidatePayload struct {
	IntegrationID model.ID `json:"integrationId"`
}
type acceptPayload struct {
	CandidateID model.ID `json:"candidateId"`
}
type candidateRecord struct {
	ID          model.ID                    `json:"id"`
	TaskID      model.ID                    `json:"taskId"`
	Integration integrationRecord           `json:"integration"`
	Policy      model.AcceptancePolicy      `json:"policy"`
	Constraints model.AcceptanceConstraints `json:"constraints"`
	ActorID     model.ID                    `json:"actorId"`
	ReceivedAt  time.Time                   `json:"receivedAt"`
}

func loadCandidate(tx rowReader, id model.ID) (candidateRecord, error) {
	var candidate candidateRecord
	var raw string
	if e := tx.QueryRow("SELECT body FROM acceptance_candidates WHERE id=?", id).Scan(&raw); e != nil {
		return candidate, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &candidate)
	return candidate, e
}
func candidateCurrent(tx rowReader, t model.Task, id model.ID) (candidateRecord, error) {
	candidate, e := loadCandidate(tx, id)
	if e != nil {
		return candidate, e
	}
	if id != t.CurrentCandidateID || candidate.TaskID != t.ID {
		return candidate, model.Conflict("stale-candidate")
	}
	if e = integrationCurrent(tx, t, candidate.Integration); e != nil {
		return candidate, e
	}
	if e = guardDecisions(tx, t, "", "acceptance"); e != nil {
		return candidate, e
	}
	return candidate, nil
}
func (s *Service) makeCandidate(tx *store.Tx, a model.Actor, c model.Command, p candidatePayload) (model.ID, int64, []byte, error) {
	task, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = effectiveIntegrator(a, task, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = mutable(task); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, task.Version); e != nil {
		return "", 0, nil, e
	}
	if e = checkVersions(tx, task, "", c.Meta, true); e != nil {
		return "", 0, nil, e
	}
	if e = guardDecisions(tx, task, "", "completion"); e != nil {
		return "", 0, nil, e
	}
	attempt, e := loadIntegration(tx, p.IntegrationID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = integrationCurrent(tx, task, attempt); e != nil {
		return "", 0, nil, e
	}
	var n int
	if e = tx.QueryRow("SELECT count(*) FROM acceptance_candidates WHERE id=?", attempt.ID).Scan(&n); e != nil {
		return "", 0, nil, e
	}
	if n != 0 {
		return "", 0, nil, model.Conflict("candidate-already-recorded")
	}
	candidate := candidateRecord{attempt.ID, task.ID, attempt, task.AcceptancePolicy, task.AcceptanceConstraints, a.ID, s.clock.Now()}
	if _, e = tx.Exec("INSERT INTO acceptance_candidates VALUES(?,?,?)", candidate.ID, task.ID, string(model.JSON(candidate))); e != nil {
		return "", 0, nil, e
	}
	task.CurrentCandidateID = candidate.ID
	task.Phase = "review"
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return candidate.ID, 1, model.JSON(candidate), nil
}
func acceptorAllowed(a model.Actor, t model.Task, m model.Mutation) error {
	switch t.AcceptancePolicy.Mode {
	case "human":
		if a.Kind != "human" {
			return model.Forbidden()
		}
	case "designated-agent":
		if a.Kind != "agent" || a.ParticipantID != t.AcceptancePolicy.AcceptorID {
			return model.Forbidden()
		}
	case "integrator":
		if e := effectiveIntegrator(a, t, m); e != nil {
			return e
		}
	default:
		return model.Forbidden()
	}
	return nil
}
func (s *Service) acceptCandidate(tx *store.Tx, a model.Actor, c model.Command, p acceptPayload) (model.ID, int64, []byte, error) {
	task, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = acceptorAllowed(a, task, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = mutable(task); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, task.Version); e != nil {
		return "", 0, nil, e
	}
	if e = checkVersions(tx, task, "", c.Meta, true); e != nil {
		return "", 0, nil, e
	}
	if c.Meta.IntegratorEpoch < 1 {
		return "", 0, nil, model.Invalid("integratorEpoch required")
	}
	if c.Meta.IntegratorEpoch != task.IntegratorEpoch {
		return "", 0, nil, model.Conflict("stale-integrator")
	}
	candidate, e := candidateCurrent(tx, task, p.CandidateID)
	if e != nil {
		return "", 0, nil, e
	}
	var count int
	if e = tx.QueryRow("SELECT count(*) FROM acceptances WHERE candidate_id=? AND actor_id=?", candidate.ID, a.ID).Scan(&count); e != nil {
		return "", 0, nil, e
	}
	if count > 0 {
		return "", 0, nil, model.Conflict("already-accepted")
	}
	id := s.ids.NewID()
	body := model.JSON(map[string]any{"id": id, "taskId": task.ID, "candidateId": candidate.ID, "actorId": a.ID, "participantId": a.ParticipantID, "actorKind": a.Kind, "sessionId": a.SessionID, "integratorEpoch": task.IntegratorEpoch, "acceptancePolicyVersion": task.AcceptancePolicyVersion, "receivedAt": s.clock.Now(), "source": "actor-reported"})
	if _, e = tx.Exec("INSERT INTO acceptances VALUES(?,?,?,?,?)", id, task.ID, candidate.ID, a.ID, string(body)); e != nil {
		return "", 0, nil, e
	}
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return id, 1, body, nil
}
func (s *Service) completeTask(tx *store.Tx, a model.Actor, c model.Command, p acceptPayload) (model.ID, int64, []byte, error) {
	task, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = effectiveIntegrator(a, task, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, task.Version); e != nil {
		return "", 0, nil, e
	}
	if task.Phase != "review" {
		return "", 0, nil, model.Conflict("not-in-review")
	}
	if e = checkVersions(tx, task, "", c.Meta, true); e != nil {
		return "", 0, nil, e
	}
	if e = guardDecisions(tx, task, "", "completion"); e != nil {
		return "", 0, nil, e
	}
	candidate, e := candidateCurrent(tx, task, p.CandidateID)
	if e != nil {
		return "", 0, nil, e
	}
	rows, e := tx.Query("SELECT body FROM acceptances WHERE candidate_id=? ORDER BY id", candidate.ID)
	if e != nil {
		return "", 0, nil, e
	}
	acceptedID := model.ID("")
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return "", 0, nil, e
		}
		var acceptance struct {
			ID, ActorID, ParticipantID, SessionID model.ID
			ActorKind                             string
			IntegratorEpoch                       int64
			AcceptancePolicyVersion               int64
		}
		if e = json.Unmarshal([]byte(raw), &acceptance); e != nil {
			rows.Close()
			return "", 0, nil, e
		}
		if acceptance.AcceptancePolicyVersion != task.AcceptancePolicyVersion || acceptance.IntegratorEpoch != task.IntegratorEpoch {
			continue
		}
		valid := false
		switch task.AcceptancePolicy.Mode {
		case "human":
			valid = acceptance.ActorKind == "human"
		case "integrator":
			valid = acceptance.ActorKind == "agent" && acceptance.ParticipantID == task.IntegratorID && acceptance.SessionID == task.IntegratorSessionID
		case "designated-agent":
			valid = acceptance.ActorKind == "agent" && acceptance.ParticipantID == task.AcceptancePolicy.AcceptorID && participantInProject(tx, acceptance.ParticipantID, task.ProjectID)
		}
		if valid {
			acceptedID = acceptance.ID
			break
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return "", 0, nil, e
	}
	if acceptedID == "" {
		return "", 0, nil, model.Conflict("acceptance-required")
	}
	completion := s.ids.NewID()
	task.Phase = "done"
	task.LastCompletionID = completion
	snapshot := model.JSON(map[string]any{"id": completion, "taskId": task.ID, "taskVersion": task.Version + 1, "candidate": candidate, "acceptanceId": acceptedID, "actorId": a.ID, "actorKind": a.Kind, "receivedAt": s.clock.Now(), "evidenceSource": "agent-reported"})
	if _, e = tx.Exec("INSERT INTO completions VALUES(?,?,?)", completion, task.ID, string(snapshot)); e != nil {
		return "", 0, nil, e
	}
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return task.ID, task.Version, model.JSON(task), nil
}
