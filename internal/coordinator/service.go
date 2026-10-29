package coordinator

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"roooooom.local/roooooom/internal/store"
	"sync"
	"time"
)

type Service struct {
	store               *store.Store
	clock               model.Clock
	ids                 model.IDSource
	observationMu       sync.Mutex
	observations        map[model.ID]observation
	timesMu             sync.Mutex
	timesCache          map[model.ID]cachedTimes
	waitRegistered      func()
	waitIdle            func()
	metrics             appruntime.Metrics
	repositoryInspector outbound.RepositoryInspector
}
type observation struct {
	LastSeen time.Time
	Waiting  int
}

func New(st *store.Store, clock model.Clock, ids model.IDSource, options ...Options) *Service {
	if clock == nil {
		clock = model.RealClock{}
	}
	if ids == nil {
		ids = model.RandomIDs{}
	}
	s := &Service{store: st, clock: clock, ids: ids, observations: map[model.ID]observation{}, timesCache: map[model.ID]cachedTimes{}}
	if len(options) > 0 {
		s.repositoryInspector = options[0].RepositoryInspector
	}
	return s
}
func (s *Service) Apply(ctx context.Context, a model.Actor, c model.Command) (receipt model.Receipt, err error) {
	start := time.Now()
	defer func() { s.metrics.Command(time.Since(start), err != nil) }()
	i := s.store.Identity()
	if c.Meta.ServiceID == "" || c.Meta.Generation == "" || c.Meta.ClientRequestID == "" || len(c.Meta.ClientRequestID) > 128 {
		return model.Receipt{}, model.Invalid("identity, generation and request ID required")
	}
	if c.Meta.ServiceID != i.ServiceID || c.Meta.Generation != i.Generation {
		return model.Receipt{}, model.Conflict("needs-reconcile")
	}
	payload, e := prepare(c)
	if e != nil {
		return model.Receipt{}, e
	}
	c.Payload = model.JSON(payload)
	digest, e := model.CanonicalHash(c)
	if e != nil {
		return model.Receipt{}, e
	}
	return s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		scope, e := commandScope(tx, c, payload)
		if e != nil {
			return model.Receipt{}, e
		}
		if e = authorize(tx, a, scope); e != nil {
			return model.Receipt{}, e
		}
		prior, e := tx.FindReceipt(a.ID, "commands", c.Meta.ClientRequestID, digest)
		if e != nil {
			return model.Receipt{}, e
		}
		if prior != nil {
			return *prior, nil
		}
		if e = activeSession(tx, a, c.Meta); e != nil {
			return model.Receipt{}, e
		}
		if e = guardReconciliation(tx, a, c, payload, scope); e != nil {
			return model.Receipt{}, e
		}
		id, version, result, e := s.execute(tx, a, c, payload)
		if e != nil {
			return model.Receipt{}, e
		}
		eventScope := string(scope)
		eventBody := map[string]any{"kind": c.Kind, "resourceId": id, "version": version, "actorId": a.ID, "actorKind": a.Kind, "sessionId": a.SessionID, "receivedAt": s.clock.Now()}
		if c.Kind == "message.send" {
			eventScope = "message:" + string(id)
			eventBody["message"] = json.RawMessage(result)
		}
		if c.Kind == "receipt.update" {
			eventScope = "receipt:" + string(a.ParticipantID)
			eventBody["receipt"] = json.RawMessage(result)
		}
		event := model.JSON(eventBody)
		seq, e := tx.AppendEvent(eventScope, event)
		if e != nil {
			return model.Receipt{}, e
		}
		if e = outbound.Enqueue(tx, i.ServiceID, i.Generation, c.Kind, id, scope, seq); e != nil {
			return model.Receipt{}, e
		}
		if e = s.enqueueNotifications(tx, c, id, payload); e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: id, Version: version, EventID: seq, Result: result}
		if e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, digest, r); e != nil {
			return model.Receipt{}, e
		}
		return r, nil
	})
}
func authorize(tx rowReader, a model.Actor, project model.ID) error {
	if a.Kind == "human" {
		return nil
	}
	if a.Kind != "agent" || project == "owner" {
		return model.Forbidden()
	}
	allowed := false
	for _, id := range a.ProjectIDs {
		if id == project {
			allowed = true
		}
	}
	if !allowed {
		return model.Forbidden()
	}
	var n int
	if e := tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", a.ParticipantID, project).Scan(&n); e != nil {
		return e
	}
	if n != 1 {
		return model.Forbidden()
	}
	return nil
}
func activeSession(tx *store.Tx, a model.Actor, m model.Mutation) error {
	if a.Kind == "human" {
		return nil
	}
	if m.SessionID == "" {
		return model.Invalid("sessionId required")
	}
	if m.SessionID != a.SessionID {
		return model.Conflict("stale-session")
	}
	var participant model.ID
	var status string
	if e := tx.QueryRow("SELECT participant_id,status FROM sessions WHERE id=?", a.SessionID).Scan(&participant, &status); e != nil {
		return model.Forbidden()
	}
	if participant != a.ParticipantID || status != "active" {
		return model.Conflict("stale-session")
	}
	return nil
}

type RepositoryPayload struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
}
type ProjectPayload struct {
	Name         string              `json:"name"`
	Repositories []RepositoryPayload `json:"repositories"`
}
type limitPayload struct {
	Limit  int    `json:"limit"`
	Reason string `json:"reason"`
}
type releasePayload struct {
	Reason   string `json:"reason"`
	ExitTask bool   `json:"exitTask"`
}
type reasonPayload struct {
	Reason string `json:"reason"`
}
type assignmentUpdate struct {
	State        *string     `json:"state,omitempty"`
	Dependencies *[]model.ID `json:"dependencies,omitempty"`
	Role         *string     `json:"role,omitempty"`
	Input        *string     `json:"input,omitempty"`
	Output       *string     `json:"output,omitempty"`
	WriteScope   *[]string   `json:"writeScope,omitempty"`
	RepoID       *model.ID   `json:"repoId,omitempty"`
	BaseRef      *string     `json:"baseRef,omitempty"`
	Reason       string      `json:"reason,omitempty"`
}

// ValidateCommandSyntax checks only the same typed shape used by the service.
// It grants no authority and does not evaluate current domain state.
func ValidateCommandSyntax(c model.Command) error {
	_, e := prepare(c)
	return e
}

func prepare(c model.Command) (any, error) {
	var p any
	var required []string
	switch c.Kind {
	case "project.create":
		p = &ProjectPayload{}
		required = []string{"name", "repositories"}
	case "request.create":
		p = &model.RequestCreate{}
		required = []string{"projectId", "text"}
	case "request.claim", "request.release", "request.handoff", "request.accept", "request.resume", "request.assign", "request.cancel":
		p = &requestTransition{}
	case "request.convert":
		p = &requestConversion{}
		required = []string{"task"}
	case "task.create":
		p = &model.TaskCreate{}
		required = []string{"projectId", "goal", "scope", "nonScope", "authorityRefs", "risk", "acceptancePolicy", "collaboratorLimit"}
	case "assignment.create":
		p = &model.AssignmentCreate{}
		required = []string{"taskId", "role", "input", "output", "writeScope", "repoId", "baseRef", "dependencies"}
	case "integrator.claim", "assignment.claim":
		p = &struct{}{}
	case "assignment.release":
		p = &releasePayload{}
		required = []string{"reason"}
	case "assignment.update":
		p = &assignmentUpdate{}
	case "collaborator.limit":
		p = &limitPayload{}
		required = []string{"limit", "reason"}
	case "task.reopen", "task.cancel":
		p = &reasonPayload{}
		required = []string{"reason"}
	case "session.checkpoint", "session.end":
		p = &checkpointPayload{}
		required = []string{"summary"}
	case "session.resume":
		p = &resumePayload{}
		required = []string{"expectedSourceSessionId", "sourceVersion", "responsibilityIds"}
	case "integrator.handoff":
		p = &handoffPayload{}
		required = []string{"fromSessionId", "toSessionId", "dispositions", "reason"}
	case "message.send":
		p = &messagePayload{}
		required = []string{"projectId", "threadId", "purpose", "priority", "body", "recipientMode", "recipientIds", "actionRequired"}
	case "receipt.update":
		p = &receiptPayload{}
		required = []string{"state"}
	case "decision.request":
		p = &decisionRequestPayload{}
		required = []string{"scopeKey", "question", "reason", "factRefs", "options", "recommendation", "recommendationReason", "impact", "blockedAssignmentIds", "blockedActions", "canContinue", "noDecisionBehavior"}
	case "decision.resolve", "decision.supersede":
		p = &resolutionPayload{}
		required = []string{"answer"}
	case "decision.followup":
		p = &followupPayload{}
		required = []string{"reason", "evidenceRefs"}
	case "decision.withdraw":
		p = &reasonPayload{}
		required = []string{"reason"}
	case "delivery.submit":
		p = &deliveryPayload{}
		required = []string{"repoId", "baseSha", "headSha", "branchRef", "workspaceRef", "summary", "evidenceRefs", "unknowns", "artifactRefs"}
	case "delivery.review":
		p = &reviewPayload{}
		required = []string{"deliveryRevisionId", "disposition", "reason"}
	case "integration.record":
		p = &integrationPayload{}
		required = []string{"deliveryRevisionIds", "heads", "checks", "summary", "result", "excludedAssignments", "untested"}
	case "acceptance.policy":
		p = &policyPayload{}
		required = []string{"policy", "reason"}
	case "acceptance.candidate":
		p = &candidatePayload{}
		required = []string{"integrationId"}
	case "acceptance.accept", "task.complete":
		p = &acceptPayload{}
		required = []string{"candidateId"}
	case "offline.reconcile":
		p = &reconcilePayload{}
		required = []string{"projectId", "localId", "originalRequestId", "originalCommand", "baseCursor", "localOccurredAt"}
	case "offline.resolve":
		p = &reconcileReviewPayload{}
		required = []string{"disposition", "reason", "evidenceRefs"}
	default:
		return nil, model.Invalid("unknown command")
	}
	if e := model.Decode(c.Payload, p); e != nil {
		return nil, e
	}
	if e := model.RequireFields(c.Payload, required...); e != nil {
		return nil, e
	}
	normalizePayload(p)
	if decision, ok := p.(*decisionRequestPayload); ok && (decision.TaskID == "") == (decision.RequestID == "") {
		return nil, model.Invalid("exactly one taskId or requestId required")
	}
	if c.Kind != "project.create" && c.Kind != "request.create" && c.Kind != "task.create" && c.Kind != "assignment.create" && c.Kind != "message.send" && c.Kind != "decision.request" && c.Kind != "offline.reconcile" && c.TargetID == "" {
		return nil, model.Invalid("targetId required")
	}
	return p, nil
}
func commandScope(tx *store.Tx, c model.Command, p any) (model.ID, error) {
	switch c.Kind {
	case "project.create":
		return "owner", nil
	case "request.create":
		return p.(*model.RequestCreate).ProjectID, nil
	case "request.claim", "request.release", "request.handoff", "request.accept", "request.resume", "request.assign", "request.cancel", "request.convert":
		r, e := loadRequest(tx, c.TargetID)
		return r.ProjectID, e
	case "task.create":
		return p.(*model.TaskCreate).ProjectID, nil
	case "message.send":
		return p.(*messagePayload).ProjectID, nil
	case "offline.reconcile":
		return p.(*reconcilePayload).ProjectID, nil
	case "offline.resolve":
		candidate, e := loadOfflineCandidate(tx, c.TargetID)
		return candidate.ProjectID, e
	case "receipt.update":
		message, e := loadMessage(tx, c.TargetID)
		return message.ProjectID, e
	case "decision.request":
		if id := p.(*decisionRequestPayload).RequestID; id != "" {
			r, e := loadRequest(tx, id)
			return r.ProjectID, e
		}
		task, e := loadTask(tx, p.(*decisionRequestPayload).TaskID)
		return task.ProjectID, e
	case "decision.resolve", "decision.supersede", "decision.followup", "decision.withdraw":
		request, e := loadDecisionRequest(tx, c.TargetID)
		return request.ProjectID, e
	case "assignment.create":
		task, e := loadTask(tx, p.(*model.AssignmentCreate).TaskID)
		return task.ProjectID, e
	case "session.checkpoint", "session.end", "session.resume":
		var project model.ID
		e := tx.QueryRow("SELECT project_id FROM tokens WHERE session_id=? AND kind='agent' LIMIT 1", c.TargetID).Scan(&project)
		if e != nil {
			return "", model.NotFound()
		}
		return project, nil
	case "assignment.claim", "assignment.release", "assignment.update", "delivery.submit", "delivery.review":
		assignment, e := loadAssignment(tx, c.TargetID)
		if e != nil {
			return "", e
		}
		task, e := loadTask(tx, assignment.TaskID)
		return task.ProjectID, e
	default:
		task, e := loadTask(tx, c.TargetID)
		return task.ProjectID, e
	}
}
func (s *Service) execute(tx *store.Tx, a model.Actor, c model.Command, p any) (model.ID, int64, []byte, error) {
	switch c.Kind {
	case "project.create":
		return s.createProject(tx, a, *p.(*ProjectPayload))
	case "request.create":
		return s.createRequest(tx, a, *p.(*model.RequestCreate))
	case "request.claim", "request.release", "request.handoff", "request.accept", "request.resume", "request.assign", "request.cancel":
		return s.transitionRequest(tx, a, c, *p.(*requestTransition))
	case "request.convert":
		return s.convertRequest(tx, a, c, *p.(*requestConversion))
	case "task.create":
		return s.createTask(tx, a, *p.(*model.TaskCreate))
	case "integrator.claim":
		return s.claimIntegrator(tx, a, c)
	case "assignment.create":
		return s.createAssignment(tx, a, c, *p.(*model.AssignmentCreate))
	case "assignment.claim":
		return s.claimAssignment(tx, a, c)
	case "assignment.release":
		return s.releaseAssignment(tx, a, c, *p.(*releasePayload))
	case "assignment.update":
		return s.updateAssignment(tx, a, c, *p.(*assignmentUpdate))
	case "session.checkpoint", "session.end":
		return s.checkpoint(tx, a, c, *p.(*checkpointPayload))
	case "session.resume":
		return s.resume(tx, a, c, *p.(*resumePayload))
	case "integrator.handoff":
		return s.handoff(tx, a, c, *p.(*handoffPayload))
	case "message.send":
		return s.sendMessage(tx, a, c, *p.(*messagePayload))
	case "receipt.update":
		return s.updateReceipt(tx, a, c, *p.(*receiptPayload))
	case "decision.request":
		return s.requestDecision(tx, a, *p.(*decisionRequestPayload), c.Meta)
	case "decision.resolve", "decision.supersede":
		return s.resolveDecision(tx, a, c, *p.(*resolutionPayload))
	case "decision.followup":
		return s.followupDecision(tx, a, c, *p.(*followupPayload))
	case "decision.withdraw":
		return s.withdrawDecision(tx, a, c, *p.(*reasonPayload))
	case "delivery.submit":
		return s.submitDelivery(tx, a, c, *p.(*deliveryPayload))
	case "delivery.review":
		return s.reviewDelivery(tx, a, c, *p.(*reviewPayload))
	case "integration.record":
		return s.recordIntegration(tx, a, c, *p.(*integrationPayload))
	case "acceptance.policy":
		return s.changePolicy(tx, a, c, *p.(*policyPayload))
	case "acceptance.candidate":
		return s.makeCandidate(tx, a, c, *p.(*candidatePayload))
	case "acceptance.accept":
		return s.acceptCandidate(tx, a, c, *p.(*acceptPayload))
	case "task.complete":
		return s.completeTask(tx, a, c, *p.(*acceptPayload))
	case "offline.reconcile":
		return s.reconcile(tx, a, *p.(*reconcilePayload))
	case "offline.resolve":
		return s.resolveOffline(tx, a, c, *p.(*reconcileReviewPayload))
	default:
		return s.changeTask(tx, a, c, p)
	}
}
