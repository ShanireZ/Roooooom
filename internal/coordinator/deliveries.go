package coordinator

import (
	"encoding/hex"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"time"
)

type artifactRef struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}
type deliveryPayload struct {
	RepoID       model.ID      `json:"repoId"`
	BaseSHA      string        `json:"baseSha"`
	HeadSHA      string        `json:"headSha"`
	BranchRef    string        `json:"branchRef"`
	WorkspaceRef string        `json:"workspaceRef"`
	Summary      string        `json:"summary"`
	EvidenceRefs []string      `json:"evidenceRefs"`
	Unknowns     []string      `json:"unknowns"`
	ArtifactRefs []artifactRef `json:"artifactRefs"`
}
type deliveryRecord struct {
	deliveryPayload
	ID                        model.ID           `json:"id"`
	TaskID                    model.ID           `json:"taskId"`
	AssignmentID              model.ID           `json:"assignmentId"`
	Revision                  int64              `json:"revision"`
	ActorID                   model.ID           `json:"actorId"`
	ActorKind                 string             `json:"actorKind"`
	SessionID                 model.ID           `json:"sessionId"`
	ClaimEpoch                int64              `json:"claimEpoch"`
	SpecVersion               int64              `json:"specVersion"`
	DecisionVersions          map[model.ID]int64 `json:"decisionVersions"`
	ReceivedAt                time.Time          `json:"receivedAt"`
	AssignmentContractVersion int64              `json:"assignmentContractVersion"`
}
type reviewPayload struct {
	DeliveryRevisionID model.ID `json:"deliveryRevisionId"`
	Disposition        string   `json:"disposition"`
	Reason             string   `json:"reason"`
}

func validSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, e := hex.DecodeString(value)
	return e == nil
}
func loadDelivery(tx rowReader, id model.ID) (deliveryRecord, error) {
	var delivery deliveryRecord
	var raw string
	if e := tx.QueryRow("SELECT body FROM deliveries WHERE id=?", id).Scan(&raw); e != nil {
		return delivery, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &delivery)
	return delivery, e
}
func sameVersions(a, b map[model.ID]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for id, v := range a {
		if b[id] != v {
			return false
		}
	}
	return true
}
func currentDelivery(tx rowReader, task model.Task, assignment model.Assignment, d deliveryRecord) error {
	current, e := currentDecisions(tx, task.ID, assignment.ID)
	if e != nil {
		return e
	}
	if d.AssignmentID != assignment.ID || d.TaskID != task.ID || d.SpecVersion != task.SpecVersion || d.AssignmentContractVersion != assignment.ContractVersion || !sameVersions(d.DecisionVersions, current) {
		return model.Conflict("stale-delivery")
	}
	if d.ID != assignment.CurrentDeliveryID {
		return model.Conflict("superseded-delivery")
	}
	return nil
}
func (s *Service) submitDelivery(tx *store.Tx, a model.Actor, c model.Command, p deliveryPayload) (model.ID, int64, []byte, error) {
	assignment, e := loadAssignment(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	task, e := loadTask(tx, assignment.TaskID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = mutable(task); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, assignment.Version); e != nil {
		return "", 0, nil, e
	}
	if e = activeClaimOwner(tx, a, task, assignment, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = checkVersions(tx, task, assignment.ID, c.Meta, false); e != nil {
		return "", 0, nil, e
	}
	if e = guardDecisions(tx, task, assignment.ID, "delivery"); e != nil {
		return "", 0, nil, e
	}
	ready, e := dependenciesReady(tx, assignment)
	if e != nil {
		return "", 0, nil, e
	}
	if !ready {
		return "", 0, nil, model.Conflict("dependency-pending")
	}
	if assignment.State != "claimed" && assignment.State != "active" && assignment.State != "submitted" && assignment.State != "accepted" {
		return "", 0, nil, model.Conflict("invalid-delivery-state")
	}
	if p.RepoID != assignment.RepoID || !validSHA(p.BaseSHA) || !validSHA(p.HeadSHA) || p.BranchRef == "" || len(p.BranchRef) > 256 || p.WorkspaceRef == "" || len(p.WorkspaceRef) > 2048 || p.Summary == "" || len(p.Summary) > 8192 || !validStrings(p.EvidenceRefs, false) || !validStrings(p.Unknowns, true) || p.ArtifactRefs == nil || len(p.ArtifactRefs) > 200 {
		return "", 0, nil, model.Invalid("exact bounded delivery references required")
	}
	for _, artifact := range p.ArtifactRefs {
		if artifact.Reference == "" || len(artifact.Reference) > 2048 || len(artifact.Digest) != 64 || !validSHA(artifact.Digest) {
			return "", 0, nil, model.Invalid("artifact SHA256 reference required")
		}
	}
	var revision int64
	if e = tx.QueryRow("SELECT COALESCE(MAX(revision),0)+1 FROM deliveries WHERE assignment_id=?", assignment.ID).Scan(&revision); e != nil {
		return "", 0, nil, e
	}
	decisions, e := currentDecisions(tx, task.ID, assignment.ID)
	if e != nil {
		return "", 0, nil, e
	}
	d := deliveryRecord{deliveryPayload: p, ID: s.ids.NewID(), TaskID: task.ID, AssignmentID: assignment.ID, Revision: revision, ActorID: a.ID, ActorKind: a.Kind, SessionID: a.SessionID, ClaimEpoch: assignment.ClaimEpoch, SpecVersion: task.SpecVersion, DecisionVersions: decisions, ReceivedAt: s.clock.Now(), AssignmentContractVersion: assignment.ContractVersion}
	if _, e = tx.Exec("INSERT INTO deliveries VALUES(?,?,?,?)", d.ID, assignment.ID, revision, string(model.JSON(d))); e != nil {
		return "", 0, nil, e
	}
	assignment.State = "submitted"
	assignment.CurrentDeliveryID = d.ID
	assignment.AcceptedDeliveryID = ""
	if e = saveAssignment(tx, &assignment); e != nil {
		return "", 0, nil, e
	}
	invalidateCandidate(&task)
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return d.ID, d.Revision, model.JSON(d), nil
}
func effectiveIntegrator(a model.Actor, t model.Task, m model.Mutation) error {
	if a.Kind != "agent" {
		return model.Forbidden()
	}
	return manager(a, t, m)
}
func (s *Service) reviewDelivery(tx *store.Tx, a model.Actor, c model.Command, p reviewPayload) (model.ID, int64, []byte, error) {
	assignment, e := loadAssignment(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	task, e := loadTask(tx, assignment.TaskID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = effectiveIntegrator(a, task, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, assignment.Version); e != nil {
		return "", 0, nil, e
	}
	if e = mutable(task); e != nil {
		return "", 0, nil, e
	}
	if e = checkVersions(tx, task, assignment.ID, c.Meta, false); e != nil {
		return "", 0, nil, e
	}
	if e = guardDecisions(tx, task, assignment.ID, "delivery"); e != nil {
		return "", 0, nil, e
	}
	d, e := loadDelivery(tx, p.DeliveryRevisionID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = currentDelivery(tx, task, assignment, d); e != nil {
		return "", 0, nil, e
	}
	if assignment.State != "submitted" {
		return "", 0, nil, model.Conflict("not-submitted")
	}
	if p.Reason == "" || len(p.Reason) > 8192 {
		return "", 0, nil, model.Invalid("review reason required")
	}
	switch p.Disposition {
	case "accepted":
		assignment.State = "accepted"
		assignment.AcceptedDeliveryID = d.ID
	case "returned":
		assignment.State = "active"
		assignment.AcceptedDeliveryID = ""
	default:
		return "", 0, nil, model.Invalid("invalid review disposition")
	}
	review := map[string]any{"id": s.ids.NewID(), "deliveryRevisionId": d.ID, "assignmentId": assignment.ID, "disposition": p.Disposition, "reason": p.Reason, "actorId": a.ID, "actorKind": a.Kind, "sessionId": a.SessionID, "integratorEpoch": task.IntegratorEpoch, "receivedAt": s.clock.Now()}
	if _, e = tx.Exec("INSERT INTO delivery_reviews VALUES(?,?,?)", review["id"], d.ID, string(model.JSON(review))); e != nil {
		return "", 0, nil, e
	}
	if e = saveAssignment(tx, &assignment); e != nil {
		return "", 0, nil, e
	}
	invalidateCandidate(&task)
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return assignment.ID, assignment.Version, model.JSON(review), nil
}
