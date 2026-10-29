package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type integrationHead struct {
	RepoID  model.ID `json:"repoId"`
	BaseSHA string   `json:"baseSha"`
	HeadSHA string   `json:"headSha"`
}
type evidenceCheck struct {
	Condition    string              `json:"condition"`
	Outcome      string              `json:"outcome"`
	HeadSHAs     map[model.ID]string `json:"headShas"`
	EvidenceRefs []string            `json:"evidenceRefs"`
}
type assignmentExclusion struct {
	AssignmentID model.ID `json:"assignmentId"`
	Reason       string   `json:"reason"`
}
type integrationPayload struct {
	DeliveryRevisionIDs []model.ID            `json:"deliveryRevisionIds"`
	Heads               []integrationHead     `json:"heads"`
	Checks              []evidenceCheck       `json:"checks"`
	Summary             string                `json:"summary"`
	Result              string                `json:"result"`
	ExcludedAssignments []assignmentExclusion `json:"excludedAssignments"`
	Untested            []string              `json:"untested"`
}
type integrationRecord struct {
	integrationPayload
	ID               model.ID           `json:"id"`
	TaskID           model.ID           `json:"taskId"`
	SpecVersion      int64              `json:"specVersion"`
	PolicyVersion    int64              `json:"acceptancePolicyVersion"`
	DecisionVersions map[model.ID]int64 `json:"decisionVersions"`
	IntegratorEpoch  int64              `json:"integratorEpoch"`
	ActorID          model.ID           `json:"actorId"`
	ActorKind        string             `json:"actorKind"`
	SessionID        model.ID           `json:"sessionId"`
	ReceivedAt       time.Time          `json:"receivedAt"`
}

func loadIntegration(tx rowReader, id model.ID) (integrationRecord, error) {
	var attempt integrationRecord
	var raw string
	if e := tx.QueryRow("SELECT body FROM integrations WHERE id=?", id).Scan(&raw); e != nil {
		return attempt, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &attempt)
	return attempt, e
}
func validateIntegrationPayload(tx rowReader, t model.Task, p integrationPayload) error {
	if p.DeliveryRevisionIDs == nil || len(p.DeliveryRevisionIDs) > 2000 || len(p.Heads) == 0 || len(p.Heads) > 20 || p.Checks == nil || len(p.Checks) > 200 || p.Summary == "" || len(p.Summary) > 8192 || p.ExcludedAssignments == nil || len(p.ExcludedAssignments) > 2000 || !validStrings(p.Untested, true) || (p.Result != "passed" && p.Result != "failed") {
		return model.Invalid("bounded integration references required")
	}
	heads := map[model.ID]string{}
	for _, head := range p.Heads {
		if heads[head.RepoID] != "" || !validSHA(head.BaseSHA) || !validSHA(head.HeadSHA) {
			return model.Invalid("unique exact repository heads required")
		}
		var n int
		if e := tx.QueryRow("SELECT count(*) FROM repositories WHERE id=? AND project_id=?", head.RepoID, t.ProjectID).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return model.Invalid("head outside task project")
		}
		heads[head.RepoID] = head.HeadSHA
	}
	conditions := map[string]bool{}
	for _, check := range p.Checks {
		if check.Condition == "" || len(check.Condition) > 4096 || conditions[check.Condition] || !validStrings(check.EvidenceRefs, false) || len(check.HeadSHAs) != len(heads) {
			return model.Invalid("unique checks bound to all integration heads required")
		}
		conditions[check.Condition] = true
		if check.Outcome != "passed" && check.Outcome != "failed" && check.Outcome != "unverified" {
			return model.Invalid("invalid check outcome")
		}
		for repo, head := range heads {
			if check.HeadSHAs[repo] != head {
				return model.Invalid("check evidence head mismatch")
			}
		}
	}
	seen := map[model.ID]bool{}
	for _, id := range p.DeliveryRevisionIDs {
		if seen[id] {
			return model.Invalid("duplicate delivery revision")
		}
		seen[id] = true
		delivery, e := loadDelivery(tx, id)
		if e != nil {
			return e
		}
		if delivery.TaskID != t.ID || heads[delivery.RepoID] == "" {
			return model.Invalid("delivery outside integration scope")
		}
	}
	seen = map[model.ID]bool{}
	for _, excluded := range p.ExcludedAssignments {
		if seen[excluded.AssignmentID] || excluded.Reason == "" || len(excluded.Reason) > 8192 {
			return model.Invalid("explicit unique exclusion reason required")
		}
		seen[excluded.AssignmentID] = true
		assignment, e := loadAssignment(tx, excluded.AssignmentID)
		if e != nil {
			return e
		}
		if assignment.TaskID != t.ID {
			return model.Invalid("excluded assignment outside task")
		}
	}
	return nil
}
func requiredInputsCurrent(tx rowReader, t model.Task, attempt integrationRecord) error {
	rows, e := tx.Query("SELECT body FROM assignments WHERE task_id=?", t.ID)
	if e != nil {
		return e
	}
	assignments := []model.Assignment{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			rows.Close()
			return e
		}
		var assignment model.Assignment
		if e = json.Unmarshal([]byte(raw), &assignment); e != nil {
			rows.Close()
			return e
		}
		assignments = append(assignments, assignment)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if len(assignments) == 0 {
		return model.Conflict("assignments-required")
	}
	excluded := map[model.ID]bool{}
	for _, entry := range attempt.ExcludedAssignments {
		excluded[entry.AssignmentID] = true
	}
	for _, assignment := range assignments {
		if !assignment.Required || excluded[assignment.ID] {
			continue
		}
		if assignment.State != "accepted" || assignment.AcceptedDeliveryID == "" || assignment.AcceptedDeliveryID != assignment.CurrentDeliveryID || !containsID(attempt.DeliveryRevisionIDs, assignment.AcceptedDeliveryID) {
			return model.Conflict("required-delivery-not-accepted")
		}
		delivery, e := loadDelivery(tx, assignment.AcceptedDeliveryID)
		if e != nil {
			return e
		}
		if e = currentDelivery(tx, t, assignment, delivery); e != nil {
			return e
		}
	}
	return nil
}
func integrationCurrent(tx rowReader, t model.Task, attempt integrationRecord) error {
	decisions, e := currentDecisions(tx, t.ID, "")
	if e != nil {
		return e
	}
	if attempt.TaskID != t.ID || attempt.SpecVersion != t.SpecVersion || attempt.PolicyVersion != t.AcceptancePolicyVersion || attempt.IntegratorEpoch != t.IntegratorEpoch || !sameVersions(attempt.DecisionVersions, decisions) || attempt.ID != t.LatestIntegrationID {
		return model.Conflict("stale-integration")
	}
	if attempt.Result != "passed" {
		return model.Conflict("integration-not-passed")
	}
	if e = requiredInputsCurrent(tx, t, attempt); e != nil {
		return e
	}
	for _, condition := range t.AcceptancePolicy.Conditions {
		passed := false
		for _, check := range attempt.Checks {
			if check.Condition == condition && check.Outcome == "passed" {
				passed = true
			}
		}
		if !passed {
			return model.Conflict("required-check-not-passed")
		}
	}
	return nil
}
func (s *Service) recordIntegration(tx *store.Tx, a model.Actor, c model.Command, p integrationPayload) (model.ID, int64, []byte, error) {
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
	if e = guardDecisions(tx, task, "", "integration"); e != nil {
		return "", 0, nil, e
	}
	if e = validateIntegrationPayload(tx, task, p); e != nil {
		return "", 0, nil, e
	}
	decisions, e := currentDecisions(tx, task.ID, "")
	if e != nil {
		return "", 0, nil, e
	}
	attempt := integrationRecord{p, s.ids.NewID(), task.ID, task.SpecVersion, task.AcceptancePolicyVersion, decisions, task.IntegratorEpoch, a.ID, a.Kind, a.SessionID, s.clock.Now()}
	if p.Result == "passed" {
		if e = requiredInputsCurrent(tx, task, attempt); e != nil {
			return "", 0, nil, e
		}
	}
	if _, e = tx.Exec("INSERT INTO integrations VALUES(?,?,?)", attempt.ID, task.ID, string(model.JSON(attempt))); e != nil {
		return "", 0, nil, e
	}
	invalidateCandidate(&task)
	task.LatestIntegrationID = attempt.ID
	if p.Result == "passed" {
		task.Phase = "integrating"
	} else {
		task.Phase = "active"
	}
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return attempt.ID, 1, model.JSON(attempt), nil
}
