package coordinator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

type rowReader interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}

func loadTask(tx rowReader, id model.ID) (model.Task, error) {
	var task model.Task
	var raw string
	e := tx.QueryRow("SELECT body FROM tasks WHERE id=?", id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return task, model.NotFound()
	}
	if e != nil {
		return task, e
	}
	e = json.Unmarshal([]byte(raw), &task)
	return task, e
}
func loadAssignment(tx rowReader, id model.ID) (model.Assignment, error) {
	var a model.Assignment
	var raw string
	e := tx.QueryRow("SELECT body FROM assignments WHERE id=?", id).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return a, model.NotFound()
	}
	if e != nil {
		return a, e
	}
	e = json.Unmarshal([]byte(raw), &a)
	return a, e
}
func nullable(id model.ID) any {
	if id == "" {
		return nil
	}
	return id
}
func saveTask(tx *store.Tx, t *model.Task) error {
	old := t.Version
	t.Version++
	result, e := tx.Exec("UPDATE tasks SET version=?,spec_version=?,phase=?,collaborator_limit=?,integrator_id=?,integrator_session_id=?,integrator_epoch=?,body=? WHERE id=? AND version=?", t.Version, t.SpecVersion, t.Phase, t.CollaboratorLimit, nullable(t.IntegratorID), nullable(t.IntegratorSessionID), t.IntegratorEpoch, string(model.JSON(t)), t.ID, old)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return model.Conflict("stale-version")
	}
	return nil
}
func saveAssignment(tx *store.Tx, a *model.Assignment) error {
	old := a.Version
	a.Version++
	result, e := tx.Exec("UPDATE assignments SET version=?,claim_participant_id=?,claim_session_id=?,claim_epoch=?,state=?,body=? WHERE id=? AND version=?", a.Version, nullable(a.ClaimParticipantID), nullable(a.ClaimSessionID), a.ClaimEpoch, a.State, string(model.JSON(a)), a.ID, old)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return model.Conflict("stale-version")
	}
	return nil
}
func participantInProject(tx *store.Tx, id, project model.ID) bool {
	var n int
	e := tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", id, project).Scan(&n)
	return e == nil && n == 1
}
func version(expected, actual int64) error {
	if expected < 1 {
		return model.Invalid("expectedVersion required")
	}
	if expected != actual {
		return model.Conflict("stale-version")
	}
	return nil
}
func manager(a model.Actor, t model.Task, m model.Mutation) error {
	if a.Kind == "human" {
		return nil
	}
	if m.IntegratorEpoch < 1 {
		return model.Invalid("integratorEpoch required")
	}
	if a.ParticipantID != t.IntegratorID || a.SessionID != t.IntegratorSessionID || m.IntegratorEpoch != t.IntegratorEpoch {
		return model.Conflict("stale-integrator")
	}
	return nil
}
func mutable(t model.Task) error {
	if t.Phase == "done" || t.Phase == "cancelled" {
		return model.Conflict("task-closed")
	}
	return nil
}
func validStrings(values []string, allowEmpty bool) bool {
	if values == nil || len(values) > 200 || (!allowEmpty && len(values) == 0) {
		return false
	}
	for _, v := range values {
		if v == "" || len(v) > 4096 {
			return false
		}
	}
	return true
}
func (s *Service) createProject(tx *store.Tx, a model.Actor, p ProjectPayload) (model.ID, int64, []byte, error) {
	if a.Kind != "human" {
		return "", 0, nil, model.Forbidden()
	}
	if p.Name == "" || len(p.Name) > 256 || p.Repositories == nil || len(p.Repositories) > 20 {
		return "", 0, nil, model.Invalid("project name and repository list required")
	}
	id := s.ids.NewID()
	if _, e := tx.Exec("INSERT INTO projects VALUES(?,1,?)", id, string(model.JSON(p))); e != nil {
		return "", 0, nil, e
	}
	repositories := []any{}
	for _, r := range p.Repositories {
		if r.Name == "" || len(r.Name) > 256 || len(r.Reference) > 2048 {
			return "", 0, nil, model.Invalid("invalid repository metadata")
		}
		repoID := s.ids.NewID()
		if _, e := tx.Exec("INSERT INTO repositories VALUES(?,?,1,?)", repoID, id, string(model.JSON(r))); e != nil {
			return "", 0, nil, e
		}
		repositories = append(repositories, map[string]any{"id": repoID, "projectId": id, "version": 1, "name": r.Name, "reference": r.Reference})
	}
	return id, 1, model.JSON(map[string]any{"id": id, "version": 1, "name": p.Name, "repositories": repositories}), nil
}
func (s *Service) createTask(tx *store.Tx, a model.Actor, p model.TaskCreate) (model.ID, int64, []byte, error) {
	if p.ProjectID == "" || p.Goal == "" || len(p.Goal) > 8192 || !validStrings(p.Scope, false) || !validStrings(p.NonScope, true) || !validStrings(p.AuthorityRefs, false) || p.Risk == "" || p.CollaboratorLimit < 0 || p.CollaboratorLimit > 64 || !validStrings(p.AcceptancePolicy.Conditions, false) {
		return "", 0, nil, model.Invalid("task scope, authority, acceptance and bounded capacity required")
	}
	if p.Risk != "normal" && p.Risk != "low" && p.Risk != "high" {
		return "", 0, nil, model.Invalid("invalid risk")
	}
	if p.AcceptancePolicy.Mode == "" {
		p.AcceptancePolicy.Mode = "integrator"
	}
	switch p.AcceptancePolicy.Mode {
	case "integrator", "human":
	case "designated-agent":
		if !participantInProject(tx, p.AcceptancePolicy.AcceptorID, p.ProjectID) {
			return "", 0, nil, model.Invalid("acceptor not in project")
		}
	default:
		return "", 0, nil, model.Invalid("invalid acceptance mode")
	}
	if a.Kind != "human" && p.AcceptancePolicy.OwnerProtected {
		return "", 0, nil, model.Forbidden()
	}
	var n int
	if e := tx.QueryRow("SELECT count(*) FROM projects WHERE id=?", p.ProjectID).Scan(&n); e != nil {
		return "", 0, nil, e
	}
	if n != 1 {
		return "", 0, nil, model.NotFound()
	}
	if p.DesignatedIntegratorID != "" && !participantInProject(tx, p.DesignatedIntegratorID, p.ProjectID) {
		return "", 0, nil, model.Invalid("designated integrator not in project")
	}
	t := model.Task{TaskCreate: p, ID: s.ids.NewID(), Version: 1, SpecVersion: 1, AcceptancePolicyVersion: 1, CreatedBy: a.ID, ActorKind: a.Kind, Phase: "ready"}
	if a.Kind == "human" {
		t.CollaboratorLimitSource = "owner"
	}
	s.initializePolicy(&t, a)
	t.DecisionVersions = map[model.ID]int64{}
	if a.Kind == "agent" && (p.DesignatedIntegratorID == "" || p.DesignatedIntegratorID == a.ParticipantID) {
		t.IntegratorID = a.ParticipantID
		t.IntegratorSessionID = a.SessionID
		t.IntegratorEpoch = 1
		t.Phase = "active"
	}
	if _, e := tx.Exec("INSERT INTO tasks(id,project_id,version,spec_version,phase,collaborator_limit,integrator_id,integrator_session_id,integrator_epoch,body) VALUES(?,?,1,1,?,?,?,?,?,?)", t.ID, t.ProjectID, t.Phase, t.CollaboratorLimit, nullable(t.IntegratorID), nullable(t.IntegratorSessionID), t.IntegratorEpoch, string(model.JSON(t))); e != nil {
		return "", 0, nil, e
	}
	if t.IntegratorID != "" {
		if _, e := tx.Exec("INSERT INTO memberships VALUES(?,?,'integrator')", t.ID, t.IntegratorID); e != nil {
			return "", 0, nil, e
		}
	}
	if e := validatePolicy(tx, t.ProjectID, t.AcceptancePolicy); e != nil {
		return "", 0, nil, e
	}
	if e := s.ensureRiskQuestion(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return t.ID, t.Version, model.JSON(t), nil
}
func (s *Service) claimIntegrator(tx *store.Tx, a model.Actor, c model.Command) (model.ID, int64, []byte, error) {
	if a.Kind != "agent" {
		return "", 0, nil, model.Forbidden()
	}
	t, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, t.Version); e != nil {
		return "", 0, nil, e
	}
	if e = mutable(t); e != nil {
		return "", 0, nil, e
	}
	if t.IntegratorID != "" {
		return "", 0, nil, model.Conflict("integrator-occupied")
	}
	if t.DesignatedIntegratorID != "" && t.DesignatedIntegratorID != a.ParticipantID {
		return "", 0, nil, model.Forbidden()
	}
	t.IntegratorID = a.ParticipantID
	t.IntegratorSessionID = a.SessionID
	t.IntegratorEpoch++
	t.Phase = "active"
	if _, e = tx.Exec("INSERT INTO memberships VALUES(?,?,'integrator')", t.ID, a.ParticipantID); e != nil {
		return "", 0, nil, e
	}
	if e = saveTask(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return t.ID, t.Version, model.JSON(t), nil
}
func (s *Service) changeTask(tx *store.Tx, a model.Actor, c model.Command, p any) (model.ID, int64, []byte, error) {
	t, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = manager(a, t, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, t.Version); e != nil {
		return "", 0, nil, e
	}
	switch c.Kind {
	case "collaborator.limit":
		p := p.(*limitPayload)
		if t.CollaboratorLimitSource == "owner" && a.Kind != "human" && p.Limit != t.CollaboratorLimit {
			return "", 0, nil, model.Forbidden()
		}
		if p.Reason == "" || p.Limit < 0 || p.Limit > 64 {
			return "", 0, nil, model.Invalid("reason and bounded limit required")
		}
		var count int
		if e = tx.QueryRow("SELECT count(*) FROM memberships WHERE task_id=? AND role='worker'", t.ID).Scan(&count); e != nil {
			return "", 0, nil, e
		}
		if p.Limit < count {
			return "", 0, nil, model.Conflict("capacity-in-use")
		}
		t.CollaboratorLimit = p.Limit
		if a.Kind == "human" {
			t.CollaboratorLimitSource = "owner"
		}
	case "task.cancel":
		if p.(*reasonPayload).Reason == "" {
			return "", 0, nil, model.Invalid("reason required")
		}
		if e = mutable(t); e != nil {
			return "", 0, nil, e
		}
		t.Phase = "cancelled"
	case "task.reopen":
		if p.(*reasonPayload).Reason == "" {
			return "", 0, nil, model.Invalid("reason required")
		}
		if t.Phase != "done" {
			return "", 0, nil, model.Conflict("not-done")
		}
		t.Phase = "review"
		t.CurrentCandidateID = ""
	default:
		return "", 0, nil, model.Invalid("unknown command")
	}
	if e = saveTask(tx, &t); e != nil {
		return "", 0, nil, e
	}
	return t.ID, t.Version, model.JSON(t), nil
}
