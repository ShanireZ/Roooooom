package coordinator

import (
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

func validateAssignment(tx *store.Tx, p model.AssignmentCreate, current model.ID) error {
	if p.TaskID == "" || p.Role == "" || len(p.Role) > 256 || p.Input == "" || len(p.Input) > 8192 || p.Output == "" || len(p.Output) > 8192 || !validStrings(p.WriteScope, true) || p.RepoID == "" || p.BaseRef == "" || len(p.BaseRef) > 256 || p.Dependencies == nil || len(p.Dependencies) > 200 {
		return model.Invalid("assignment contract required")
	}
	var n int
	if e := tx.QueryRow("SELECT count(*) FROM repositories r JOIN tasks t ON t.project_id=r.project_id WHERE r.id=? AND t.id=?", p.RepoID, p.TaskID).Scan(&n); e != nil {
		return e
	}
	if n != 1 {
		return model.Invalid("repository not in task project")
	}
	visited := map[model.ID]bool{}
	var walk func(model.ID) error
	walk = func(id model.ID) error {
		if id == current {
			return model.Invalid("dependency cycle")
		}
		if visited[id] {
			return nil
		}
		visited[id] = true
		a, e := loadAssignment(tx, id)
		if e != nil || a.TaskID != p.TaskID {
			return model.Invalid("unknown or cross-task dependency")
		}
		for _, dependency := range a.Dependencies {
			if e = walk(dependency); e != nil {
				return e
			}
		}
		return nil
	}
	for _, dependency := range p.Dependencies {
		if e := walk(dependency); e != nil {
			return e
		}
	}
	return nil
}
func dependenciesReady(tx rowReader, a model.Assignment) (bool, error) {
	for _, id := range a.Dependencies {
		dependency, e := loadAssignment(tx, id)
		if e != nil {
			return false, e
		}
		if dependency.State != "accepted" {
			return false, nil
		}
		task, e := loadTask(tx, dependency.TaskID)
		if e != nil {
			return false, e
		}
		if dependency.AcceptedDeliveryID == "" || dependency.AcceptedDeliveryID != dependency.CurrentDeliveryID {
			return false, nil
		}
		delivery, e := loadDelivery(tx, dependency.AcceptedDeliveryID)
		if e != nil {
			return false, nil
		}
		if e = currentDelivery(tx, task, dependency, delivery); e != nil {
			return false, nil
		}
		if e = guardDecisions(tx, task, dependency.ID, "completion"); e != nil {
			return false, nil
		}
	}
	return true, nil
}
func (s *Service) createAssignment(tx *store.Tx, a model.Actor, c model.Command, p model.AssignmentCreate) (model.ID, int64, []byte, error) {
	t, e := loadTask(tx, p.TaskID)
	if e != nil {
		return "", 0, nil, e
	}
	if t.IntegratorID == "" {
		return "", 0, nil, model.Conflict("integrator-required")
	}
	if e = manager(a, t, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = mutable(t); e != nil {
		return "", 0, nil, e
	}
	id := s.ids.NewID()
	if e = validateAssignment(tx, p, id); e != nil {
		return "", 0, nil, e
	}
	var count int
	if e = tx.QueryRow("SELECT count(*) FROM assignments WHERE task_id=?", t.ID).Scan(&count); e != nil {
		return "", 0, nil, e
	}
	if count >= 2000 {
		return "", 0, nil, model.Conflict("assignment-capacity")
	}
	assignment := model.Assignment{AssignmentCreate: p, ID: id, Version: 1, State: "open", Required: true, ContractVersion: 1}
	if _, e = tx.Exec("INSERT INTO assignments(id,task_id,version,state,body) VALUES(?,?,1,'open',?)", id, t.ID, string(model.JSON(assignment))); e != nil {
		return "", 0, nil, e
	}
	if e = saveTask(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return id, 1, model.JSON(assignment), nil
}
func (s *Service) claimAssignment(tx *store.Tx, a model.Actor, c model.Command) (model.ID, int64, []byte, error) {
	if a.Kind != "agent" {
		return "", 0, nil, model.Forbidden()
	}
	assignment, e := loadAssignment(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	t, e := loadTask(tx, assignment.TaskID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = mutable(t); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, assignment.Version); e != nil {
		return "", 0, nil, e
	}
	if t.IntegratorID == "" {
		return "", 0, nil, model.Conflict("integrator-required")
	}
	if assignment.ClaimParticipantID != "" || assignment.State != "open" {
		return "", 0, nil, model.Conflict("claim-occupied")
	}
	var exists, count int
	if e = tx.QueryRow("SELECT count(*) FROM memberships WHERE task_id=? AND participant_id=?", t.ID, a.ParticipantID).Scan(&exists); e != nil {
		return "", 0, nil, e
	}
	if exists == 0 {
		if e = tx.QueryRow("SELECT count(*) FROM memberships WHERE task_id=? AND role='worker'", t.ID).Scan(&count); e != nil {
			return "", 0, nil, e
		}
		if count >= t.CollaboratorLimit {
			return "", 0, nil, model.Conflict("collaborator-capacity")
		}
		if _, e = tx.Exec("INSERT INTO memberships VALUES(?,?,'worker')", t.ID, a.ParticipantID); e != nil {
			return "", 0, nil, e
		}
	} else if a.ParticipantID == t.IntegratorID && a.SessionID != t.IntegratorSessionID {
		return "", 0, nil, model.Conflict("stale-integrator-session")
	}
	assignment.ClaimParticipantID = a.ParticipantID
	assignment.ClaimSessionID = a.SessionID
	assignment.ClaimEpoch++
	assignment.State = "claimed"
	if e = saveAssignment(tx, &assignment); e != nil {
		return "", 0, nil, e
	}
	if e = saveTask(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return assignment.ID, assignment.Version, model.JSON(assignment), nil
}
func claimOwner(a model.Actor, assignment model.Assignment, m model.Mutation) error {
	if m.ClaimEpoch < 1 {
		return model.Invalid("claimEpoch required")
	}
	if a.Kind != "agent" || a.ParticipantID != assignment.ClaimParticipantID || a.SessionID != assignment.ClaimSessionID || m.ClaimEpoch != assignment.ClaimEpoch {
		return model.Conflict("stale-claim")
	}
	return nil
}
func activeClaimOwner(tx rowReader, a model.Actor, task model.Task, assignment model.Assignment, m model.Mutation) error {
	if e := claimOwner(a, assignment, m); e != nil {
		return e
	}
	var member int
	if e := tx.QueryRow("SELECT count(*) FROM memberships WHERE task_id=? AND participant_id=?", task.ID, a.ParticipantID).Scan(&member); e != nil {
		return e
	}
	if member != 1 {
		return model.Conflict("task-membership-required")
	}
	if a.ParticipantID == task.IntegratorID && a.SessionID != task.IntegratorSessionID {
		return model.Conflict("stale-integrator-session")
	}
	return nil
}
func (s *Service) releaseAssignment(tx *store.Tx, a model.Actor, c model.Command, p releasePayload) (model.ID, int64, []byte, error) {
	assignment, e := loadAssignment(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	t, e := loadTask(tx, assignment.TaskID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, assignment.Version); e != nil {
		return "", 0, nil, e
	}
	if p.Reason == "" {
		return "", 0, nil, model.Invalid("release reason required")
	}
	if assignment.ClaimParticipantID == "" {
		return "", 0, nil, model.Conflict("not-claimed")
	}
	if e = activeClaimOwner(tx, a, t, assignment, c.Meta); e != nil {
		if e = manager(a, t, c.Meta); e != nil {
			return "", 0, nil, e
		}
		if c.Meta.ClaimEpoch < 1 {
			return "", 0, nil, model.Invalid("claimEpoch required")
		}
		if c.Meta.ClaimEpoch != assignment.ClaimEpoch {
			return "", 0, nil, model.Conflict("stale-claim")
		}
	}
	participant := assignment.ClaimParticipantID
	assignment.ClaimParticipantID = ""
	assignment.ClaimSessionID = ""
	assignment.ClaimEpoch++
	assignment.State = "open"
	if e = saveAssignment(tx, &assignment); e != nil {
		return "", 0, nil, e
	}
	if p.ExitTask && participant != t.IntegratorID {
		var count int
		if e = tx.QueryRow("SELECT count(*) FROM assignments WHERE task_id=? AND claim_participant_id=?", t.ID, participant).Scan(&count); e != nil {
			return "", 0, nil, e
		}
		if count != 0 {
			return "", 0, nil, model.Conflict("other-claims-held")
		}
		if _, e = tx.Exec("DELETE FROM memberships WHERE task_id=? AND participant_id=? AND role='worker'", t.ID, participant); e != nil {
			return "", 0, nil, e
		}
	}
	if e = saveTask(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return assignment.ID, assignment.Version, model.JSON(assignment), nil
}
func (s *Service) updateAssignment(tx *store.Tx, a model.Actor, c model.Command, p assignmentUpdate) (model.ID, int64, []byte, error) {
	assignment, e := loadAssignment(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	t, e := loadTask(tx, assignment.TaskID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = mutable(t); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, assignment.Version); e != nil {
		return "", 0, nil, e
	}
	if assignment.State == "cancelled" {
		return "", 0, nil, model.Conflict("assignment-cancelled")
	}
	if p.State != nil && *p.State == "cancelled" {
		return s.cancelAssignment(tx, a, c, p, t, assignment)
	}
	changed := false
	if p.Dependencies != nil || p.Role != nil || p.Input != nil || p.Output != nil || p.WriteScope != nil || p.RepoID != nil || p.BaseRef != nil {
		if e = manager(a, t, c.Meta); e != nil {
			return "", 0, nil, e
		}
		if p.Dependencies != nil {
			assignment.Dependencies = *p.Dependencies
		}
		if p.Role != nil {
			assignment.Role = *p.Role
		}
		if p.Input != nil {
			assignment.Input = *p.Input
		}
		if p.Output != nil {
			assignment.Output = *p.Output
		}
		if p.WriteScope != nil {
			assignment.WriteScope = *p.WriteScope
		}
		if p.RepoID != nil {
			assignment.RepoID = *p.RepoID
		}
		if p.BaseRef != nil {
			assignment.BaseRef = *p.BaseRef
		}
		if e = validateAssignment(tx, assignment.AssignmentCreate, assignment.ID); e != nil {
			return "", 0, nil, e
		}
		assignment.ContractVersion++
		assignment.AcceptedDeliveryID = ""
		if assignment.ClaimParticipantID != "" {
			assignment.State = "claimed"
		} else {
			assignment.State = "open"
		}
		invalidateCandidate(&t)
		changed = true
	}
	if p.State != nil {
		if e = activeClaimOwner(tx, a, t, assignment, c.Meta); e != nil {
			return "", 0, nil, e
		}
		if assignment.State != "claimed" && assignment.State != "active" && assignment.State != "blocked" {
			return "", 0, nil, model.Conflict("invalid-assignment-state")
		}
		if *p.State != "active" && *p.State != "blocked" {
			return "", 0, nil, model.Invalid("use delivery or release for this state")
		}
		if *p.State == "active" {
			if e = checkVersions(tx, t, assignment.ID, c.Meta, false); e != nil {
				return "", 0, nil, e
			}
			if e = guardDecisions(tx, t, assignment.ID, "implementation"); e != nil {
				return "", 0, nil, e
			}
			ready, e := dependenciesReady(tx, assignment)
			if e != nil {
				return "", 0, nil, e
			}
			if !ready {
				return "", 0, nil, model.Conflict("dependency-pending")
			}
		}
		assignment.State = *p.State
		changed = true
	}
	if !changed {
		return "", 0, nil, model.Invalid("assignment change required")
	}
	if e = saveAssignment(tx, &assignment); e != nil {
		return "", 0, nil, e
	}
	if e = saveTask(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return assignment.ID, assignment.Version, model.JSON(assignment), nil
}

func (s *Service) cancelAssignment(tx *store.Tx, a model.Actor, c model.Command, p assignmentUpdate, task model.Task, assignment model.Assignment) (model.ID, int64, []byte, error) {
	if e := manager(a, task, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if assignment.State == "accepted" {
		return "", 0, nil, model.Conflict("accepted-assignment-is-historical")
	}
	if p.Reason == "" || len(p.Reason) > 8192 || p.Dependencies != nil || p.Role != nil || p.Input != nil || p.Output != nil || p.WriteScope != nil || p.RepoID != nil || p.BaseRef != nil {
		return "", 0, nil, model.Invalid("cancellation requires a reason and no contract edits")
	}
	if c.Meta.ClaimEpoch > 0 && c.Meta.ClaimEpoch != assignment.ClaimEpoch {
		return "", 0, nil, model.Conflict("stale-claim")
	}
	assignment.State = "cancelled"
	assignment.CancellationReason = p.Reason
	assignment.ClaimParticipantID = ""
	assignment.ClaimSessionID = ""
	assignment.ClaimEpoch++
	assignment.ContractVersion++
	invalidateCandidate(&task)
	if e := saveAssignment(tx, &assignment); e != nil {
		return "", 0, nil, e
	}
	if e := saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return assignment.ID, assignment.Version, model.JSON(assignment), nil
}
