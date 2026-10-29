package coordinator

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"strconv"
	"strings"
	"time"
)

type decisionOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Impact  string `json:"impact"`
	Outcome string `json:"outcome,omitempty"`
}
type decisionRequestPayload struct {
	ActionKind           string                 `json:"actionKind,omitempty"`
	TaskID               model.ID               `json:"taskId,omitempty"`
	RequestID            model.ID               `json:"requestId,omitempty"`
	ScopeKey             string                 `json:"scopeKey"`
	Question             string                 `json:"question"`
	Reason               string                 `json:"reason"`
	FactRefs             []string               `json:"factRefs"`
	Options              []decisionOption       `json:"options"`
	Recommendation       string                 `json:"recommendation"`
	RecommendationReason string                 `json:"recommendationReason"`
	Impact               string                 `json:"impact"`
	BlockedAssignmentIDs []model.ID             `json:"blockedAssignmentIds"`
	BlockedActions       []string               `json:"blockedActions"`
	CanContinue          []string               `json:"canContinue"`
	NoDecisionBehavior   string                 `json:"noDecisionBehavior"`
	Deadline             *time.Time             `json:"deadline,omitempty"`
	ExternalProposal     *externalAuthorization `json:"externalProposal,omitempty"`
	SpecProposal         *specAmendment         `json:"specProposal,omitempty"`
}
type decisionRequestRecord struct {
	decisionRequestPayload
	ID                  model.ID  `json:"id"`
	ProjectID           model.ID  `json:"projectId"`
	Version             int64     `json:"version"`
	State               string    `json:"state"`
	ActorID             model.ID  `json:"actorId"`
	ActorKind           string    `json:"actorKind"`
	ReceivedAt          time.Time `json:"receivedAt"`
	CurrentDecisionID   model.ID  `json:"currentDecisionId,omitempty"`
	DecisionVersion     int64     `json:"decisionVersion"`
	FollowUpRequired    bool      `json:"followUpRequired"`
	FollowUpComplete    bool      `json:"followUpComplete"`
	FollowUpEvidence    []string  `json:"followUpEvidence,omitempty"`
	SpecVersion         int64     `json:"specVersion"`
	ApprovedSpecVersion int64     `json:"approvedSpecVersion"`
	ResolutionOutcome   string    `json:"resolutionOutcome,omitempty"`
}
type externalAuthorization struct {
	Action       string    `json:"action"`
	Target       string    `json:"target"`
	Scope        string    `json:"scope"`
	OriginalText string    `json:"originalText"`
	ExpiresAt    time.Time `json:"expiresAt"`
	MaxUses      int       `json:"maxUses"`
}
type resolutionPayload struct {
	Answer                    string                 `json:"answer"`
	OptionID                  string                 `json:"optionId,omitempty"`
	SupersedesDecisionID      model.ID               `json:"supersedesDecisionId,omitempty"`
	AuthorityFollowUpRequired bool                   `json:"authorityFollowUpRequired,omitempty"`
	ExternalAuthorization     *externalAuthorization `json:"externalAuthorization,omitempty"`
	Outcome                   string                 `json:"outcome,omitempty"`
	SpecAmendment             *specAmendment         `json:"specAmendment,omitempty"`
}
type specAmendment struct {
	Goal          string   `json:"goal"`
	Scope         []string `json:"scope"`
	NonScope      []string `json:"nonScope"`
	AuthorityRefs []string `json:"authorityRefs"`
	Risk          string   `json:"risk"`
}
type followupPayload struct {
	Reason       string   `json:"reason"`
	EvidenceRefs []string `json:"evidenceRefs"`
}

func loadDecisionRequest(tx rowReader, id model.ID) (decisionRequestRecord, error) {
	var request decisionRequestRecord
	var raw string
	if e := tx.QueryRow("SELECT body FROM decision_requests WHERE id=?", id).Scan(&raw); e != nil {
		return request, model.NotFound()
	}
	e := json.Unmarshal([]byte(raw), &request)
	return request, e
}
func saveDecisionRequest(tx *store.Tx, r *decisionRequestRecord) error {
	old := r.Version
	r.Version++
	result, e := tx.Exec("UPDATE decision_requests SET version=?,state=?,body=? WHERE id=? AND version=?", r.Version, r.State, string(model.JSON(r)), r.ID, old)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return model.Conflict("stale-decision-request")
	}
	return nil
}
func (s *Service) requestDecision(tx *store.Tx, a model.Actor, p decisionRequestPayload, meta ...model.Mutation) (model.ID, int64, []byte, error) {
	if draft := p.ExternalProposal; draft != nil {
		if !containsString(p.BlockedActions, "external-action") || draft.Target == "" || len(draft.Target) > 2048 || draft.Scope == "" || len(draft.Scope) > 8192 || draft.OriginalText == "" || len(draft.OriginalText) > 8192 || draft.MaxUses < 1 || draft.MaxUses > 100 || draft.ExpiresAt.IsZero() {
			return "", 0, nil, model.Invalid("complete bounded external proposal required")
		}
		switch draft.Action {
		case "push", "pr", "merge", "release", "deploy":
		default:
			return "", 0, nil, model.Invalid("invalid proposed external action")
		}
	}
	if draft := p.SpecProposal; draft != nil {
		if !containsString(p.BlockedActions, "spec-change") || draft.Goal == "" || len(draft.Goal) > 8192 || !validStrings(draft.Scope, false) || !validStrings(draft.NonScope, true) || !validStrings(draft.AuthorityRefs, false) || (draft.Risk != "low" && draft.Risk != "normal" && draft.Risk != "high") {
			return "", 0, nil, model.Invalid("complete specification proposal required")
		}
	}
	if p.ActionKind != "" && p.ActionKind != "decision-pending" && p.ActionKind != "owner-blocker" {
		return "", 0, nil, model.Invalid("invalid owner action kind")
	}
	if strings.HasPrefix(p.ScopeKey, "system:") && a.Kind != "system" {
		return "", 0, nil, model.Forbidden()
	}
	task, intake, e := decisionSubject(tx, p.TaskID, p.RequestID)
	if e != nil {
		return "", 0, nil, e
	}
	if intake.ID != "" {
		if intake.State != "preparing" || len(meta) == 0 {
			return "", 0, nil, model.Conflict("request-not-preparing")
		}
		if e = preparationOwner(a, intake, meta[0]); e != nil {
			return "", 0, nil, e
		}
		if e = version(meta[0].ExpectedVersion, intake.Version); e != nil {
			return "", 0, nil, e
		}
		if len(p.BlockedAssignmentIDs) != 0 || !containsString(p.BlockedActions, "request-convert") {
			return "", 0, nil, model.Invalid("intake question must block conversion without assignment references")
		}
	}
	if p.ScopeKey == "" || len(p.ScopeKey) > 128 || p.Question == "" || len(p.Question) > 8192 || p.Reason == "" || len(p.Reason) > 8192 || !validStrings(p.FactRefs, true) || len(p.Options) < 2 || len(p.Options) > 3 || p.RecommendationReason == "" || p.Impact == "" || p.NoDecisionBehavior == "" || !validStrings(p.CanContinue, true) || p.BlockedAssignmentIDs == nil || p.BlockedActions == nil || len(p.BlockedAssignmentIDs) > 2000 || len(p.BlockedActions) > 8 {
		return "", 0, nil, model.Invalid("complete scoped decision request required")
	}
	options := map[string]bool{}
	for _, option := range p.Options {
		if option.Outcome != "" && option.Outcome != "answered" && option.Outcome != "approved" && option.Outcome != "rejected" && option.Outcome != "revise" {
			return "", 0, nil, model.Invalid("invalid option outcome")
		}
		if option.ID == "" || len(option.ID) > 128 || option.Label == "" || len(option.Label) > 1024 || option.Impact == "" || len(option.Impact) > 4096 || options[option.ID] {
			return "", 0, nil, model.Invalid("invalid decision options")
		}
		options[option.ID] = true
	}
	if !options[p.Recommendation] {
		return "", 0, nil, model.Invalid("recommendation must identify an option")
	}
	for _, id := range p.BlockedAssignmentIDs {
		assignment, e := loadAssignment(tx, id)
		if e != nil || assignment.TaskID != task.ID {
			return "", 0, nil, model.Invalid("blocked assignment outside task")
		}
	}
	for _, action := range p.BlockedActions {
		switch action {
		case "implementation", "delivery", "integration", "acceptance", "completion", "spec-change", "external-action":
		case "request-convert":
			if intake.ID == "" {
				return "", 0, nil, model.Invalid("request-convert requires intake subject")
			}
		default:
			return "", 0, nil, model.Invalid("invalid blocked action")
		}
	}
	var existing model.ID
	if e = tx.QueryRow("SELECT id FROM decision_requests WHERE (task_id=? OR request_id=?) AND scope_key=? AND state='pending'", nullable(task.ID), nullable(intake.ID), p.ScopeKey).Scan(&existing); e == nil {
		r, e := loadDecisionRequest(tx, existing)
		return r.ID, r.Version, model.JSON(r), e
	}
	project := task.ProjectID
	if intake.ID != "" {
		project = intake.ProjectID
	}
	r := decisionRequestRecord{decisionRequestPayload: p, ID: s.ids.NewID(), ProjectID: project, Version: 1, State: "pending", ActorID: a.ID, ActorKind: a.Kind, ReceivedAt: s.clock.Now(), SpecVersion: task.SpecVersion}
	if _, e = tx.Exec("INSERT INTO decision_requests(id,project_id,task_id,version,scope_key,state,body,request_id) VALUES(?,?,?,1,?,'pending',?,?)", r.ID, r.ProjectID, nullable(r.TaskID), r.ScopeKey, string(model.JSON(r)), nullable(r.RequestID)); e != nil {
		return "", 0, nil, e
	}
	if task.ID != "" {
		invalidateCandidate(&task)
		e = saveTask(tx, &task)
	} else {
		e = saveRequest(tx, &intake)
	}
	if e != nil {
		return "", 0, nil, e
	}
	return r.ID, 1, model.JSON(r), nil
}
func (s *Service) withdrawDecision(tx *store.Tx, a model.Actor, c model.Command, p reasonPayload) (model.ID, int64, []byte, error) {
	if a.Kind != "human" {
		return "", 0, nil, model.Forbidden()
	}
	request, e := loadDecisionRequest(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, request.Version); e != nil {
		return "", 0, nil, e
	}
	if request.State != "pending" {
		return "", 0, nil, model.Conflict("decision-not-pending")
	}
	if p.Reason == "" || len(p.Reason) > 8192 {
		return "", 0, nil, model.Invalid("withdraw reason required")
	}
	request.State = "withdrawn"
	if e = saveDecisionRequest(tx, &request); e != nil {
		return "", 0, nil, e
	}
	if request.RequestID != "" {
		intake, e := loadRequest(tx, request.RequestID)
		if e != nil {
			return "", 0, nil, e
		}
		if e = saveRequest(tx, &intake); e != nil {
			return "", 0, nil, e
		}
	}
	return request.ID, request.Version, model.JSON(map[string]any{"request": request, "reason": p.Reason, "actorKind": "human"}), nil
}
func (s *Service) resolveDecision(tx *store.Tx, a model.Actor, c model.Command, p resolutionPayload) (model.ID, int64, []byte, error) {
	if a.Kind != "human" {
		return "", 0, nil, model.Forbidden()
	}
	request, e := loadDecisionRequest(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, request.Version); e != nil {
		return "", 0, nil, e
	}
	if p.Answer == "" || len(p.Answer) > 8192 {
		return "", 0, nil, model.Invalid("explicit answer required")
	}
	if c.Kind == "decision.resolve" {
		if request.State != "pending" || p.SupersedesDecisionID != "" {
			return "", 0, nil, model.Conflict("decision-not-pending")
		}
	} else if request.State != "resolved" || p.SupersedesDecisionID != request.CurrentDecisionID {
		return "", 0, nil, model.Conflict("decision-changed")
	}
	outcome := p.Outcome
	if p.OptionID != "" {
		found := false
		for _, option := range request.Options {
			if option.ID == p.OptionID {
				found = true
				if option.Outcome != "" {
					if outcome != "" && outcome != option.Outcome {
						return "", 0, nil, model.Invalid("option outcome mismatch")
					}
					outcome = option.Outcome
				}
			}
		}
		if !found {
			return "", 0, nil, model.Invalid("unknown option")
		}
	}
	if outcome == "" {
		outcome = "answered"
	}
	if outcome != "answered" && outcome != "approved" && outcome != "rejected" && outcome != "revise" {
		return "", 0, nil, model.Invalid("invalid resolution outcome")
	}
	if auth := p.ExternalAuthorization; auth != nil {
		if outcome != "approved" || !containsString(request.BlockedActions, "external-action") || auth.Action == "" || auth.Target == "" || auth.Scope == "" || auth.OriginalText == "" || !s.clock.Now().Before(auth.ExpiresAt) || auth.MaxUses < 1 || auth.MaxUses > 100 {
			return "", 0, nil, model.Invalid("scoped external authorization required")
		}
		switch auth.Action {
		case "push", "pr", "merge", "release", "deploy":
		default:
			return "", 0, nil, model.Invalid("invalid external action")
		}
	}
	task, intake, e := decisionSubject(tx, request.TaskID, request.RequestID)
	if e != nil {
		return "", 0, nil, e
	}
	questionSpec := request.SpecVersion
	if intake.ID != "" && questionSpec == 0 {
		questionSpec = intake.ConvertedSpecVersion
	}
	if task.ID != "" && questionSpec != task.SpecVersion && request.ApprovedSpecVersion != task.SpecVersion {
		return "", 0, nil, model.Conflict("question-scope-changed")
	}
	if intake.State == "cancelled" {
		return "", 0, nil, model.Conflict("request-closed")
	}
	if amendment := p.SpecAmendment; amendment != nil {
		if task.ID == "" {
			return "", 0, nil, model.Invalid("spec amendment requires a task")
		}
		if !containsString(request.BlockedActions, "spec-change") || amendment.Goal == "" || len(amendment.Goal) > 8192 || !validStrings(amendment.Scope, false) || !validStrings(amendment.NonScope, true) || !validStrings(amendment.AuthorityRefs, false) || (amendment.Risk != "normal" && amendment.Risk != "low" && amendment.Risk != "high") {
			return "", 0, nil, model.Invalid("scoped specification amendment required")
		}
		if outcome == "rejected" || outcome == "revise" {
			return "", 0, nil, model.Invalid("amendment requires an affirmative owner decision")
		}
		task.Goal = amendment.Goal
		task.Scope = amendment.Scope
		task.NonScope = amendment.NonScope
		task.AuthorityRefs = amendment.AuthorityRefs
		task.Risk = amendment.Risk
		task.SpecVersion++
	}
	id := s.ids.NewID()
	request.DecisionVersion++
	request.CurrentDecisionID = id
	request.State = "resolved"
	request.ApprovedSpecVersion = task.SpecVersion
	request.ResolutionOutcome = outcome
	request.FollowUpRequired = p.AuthorityFollowUpRequired
	request.FollowUpComplete = !p.AuthorityFollowUpRequired
	body := model.JSON(map[string]any{"id": id, "requestId": request.ID, "version": request.DecisionVersion, "answer": p.Answer, "optionId": p.OptionID, "supersedesDecisionId": p.SupersedesDecisionID, "actorId": a.ID, "actorKind": "human", "receivedAt": s.clock.Now(), "externalAuthorization": p.ExternalAuthorization, "authorityFollowUpRequired": p.AuthorityFollowUpRequired, "outcome": outcome, "specAmendment": p.SpecAmendment, "specVersion": task.SpecVersion})
	if _, e = tx.Exec("INSERT INTO decisions VALUES(?,?,?,?)", id, request.ID, request.DecisionVersion, string(body)); e != nil {
		return "", 0, nil, e
	}
	if e = saveDecisionRequest(tx, &request); e != nil {
		return "", 0, nil, e
	}
	if task.ID != "" {
		invalidateCandidate(&task)
		if task.DecisionVersions == nil {
			task.DecisionVersions = map[model.ID]int64{}
		}
		task.DecisionVersions[request.ID] = request.DecisionVersion
		if e = saveTask(tx, &task); e != nil {
			return "", 0, nil, e
		}
		if e = s.ensureRiskQuestion(tx, &task); e != nil {
			return "", 0, nil, e
		}
	}
	if intake.ID != "" {
		if e = saveRequest(tx, &intake); e != nil {
			return "", 0, nil, e
		}
	}
	return request.ID, request.Version, model.JSON(map[string]any{"request": request, "decision": json.RawMessage(body)}), nil
}
func (s *Service) followupDecision(tx *store.Tx, a model.Actor, c model.Command, p followupPayload) (model.ID, int64, []byte, error) {
	request, e := loadDecisionRequest(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	task, intake, e := decisionSubject(tx, request.TaskID, request.RequestID)
	if e != nil {
		return "", 0, nil, e
	}
	if task.ID != "" {
		e = manager(a, task, c.Meta)
	} else if a.Kind != "human" {
		e = preparationOwner(a, intake, c.Meta)
	}
	if e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, request.Version); e != nil {
		return "", 0, nil, e
	}
	if request.State != "resolved" || !request.FollowUpRequired || request.FollowUpComplete {
		return "", 0, nil, model.Conflict("followup-not-pending")
	}
	if p.Reason == "" || !validStrings(p.EvidenceRefs, false) {
		return "", 0, nil, model.Invalid("followup evidence required")
	}
	if task.ID != "" {
		e = checkVersions(tx, task, "", c.Meta, true)
	}
	if e != nil {
		return "", 0, nil, e
	}
	request.FollowUpComplete = true
	request.FollowUpEvidence = p.EvidenceRefs
	if e = saveDecisionRequest(tx, &request); e != nil {
		return "", 0, nil, e
	}
	if intake.ID != "" {
		if e = saveRequest(tx, &intake); e != nil {
			return "", 0, nil, e
		}
	}
	return request.ID, request.Version, model.JSON(request), nil
}
func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func containsID(values []model.ID, value model.ID) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func decisionRequests(tx rowReader, task model.ID) ([]decisionRequestRecord, error) {
	rows, e := tx.Query("SELECT body FROM decision_requests WHERE task_id=? OR request_id IN(SELECT id FROM work_requests WHERE task_id=?) ORDER BY id", task, task)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []decisionRequestRecord{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var request decisionRequestRecord
		if e = json.Unmarshal([]byte(raw), &request); e != nil {
			return nil, e
		}
		result = append(result, request)
	}
	return result, rows.Err()
}
func currentDecisions(tx rowReader, task, assignment model.ID) (map[model.ID]int64, error) {
	requests, e := decisionRequests(tx, task)
	if e != nil {
		return nil, e
	}
	result := map[model.ID]int64{}
	for _, r := range requests {
		if r.State == "resolved" && (assignment == "" || len(r.BlockedAssignmentIDs) == 0 || containsID(r.BlockedAssignmentIDs, assignment)) {
			result[r.ID] = r.DecisionVersion
		}
	}
	return result, nil
}
func checkVersions(tx rowReader, t model.Task, assignment model.ID, m model.Mutation, policy bool) error {
	if m.SpecVersion < 1 {
		return model.Invalid("specVersion required")
	}
	if m.SpecVersion != t.SpecVersion {
		return model.Conflict("stale-spec")
	}
	if policy {
		if m.AcceptancePolicyVersion < 1 {
			return model.Invalid("acceptancePolicyVersion required")
		}
		if m.AcceptancePolicyVersion != t.AcceptancePolicyVersion {
			return model.Conflict("stale-policy")
		}
	}
	current, e := currentDecisions(tx, t.ID, assignment)
	if e != nil {
		return e
	}
	for id, version := range current {
		if m.DecisionVersions[id] != version {
			return model.Conflict("stale-decision")
		}
	}
	all, e := currentDecisions(tx, t.ID, "")
	if e != nil {
		return e
	}
	for id, version := range m.DecisionVersions {
		if all[id] != version {
			return model.Conflict("stale-decision")
		}
	}
	return nil
}
func guardDecisions(tx rowReader, t model.Task, assignment model.ID, action string) error {
	requests, e := decisionRequests(tx, t.ID)
	if e != nil {
		return e
	}
	if t.Risk == "high" && (action == "implementation" || action == "delivery" || action == "integration" || action == "acceptance" || action == "completion") {
		approved := false
		for _, r := range requests {
			if strings.HasPrefix(r.ScopeKey, "system:risk:") && r.State == "resolved" && r.ResolutionOutcome == "approved" && r.ApprovedSpecVersion == t.SpecVersion {
				approved = true
			}
		}
		if !approved {
			return model.Conflict("owner-risk-approval-required")
		}
	}
	for _, r := range requests {
		if strings.HasPrefix(r.ScopeKey, "system:risk:") && r.SpecVersion != t.SpecVersion && r.ApprovedSpecVersion != t.SpecVersion {
			continue
		}
		if r.State != "pending" && !(r.FollowUpRequired && !r.FollowUpComplete) && r.ResolutionOutcome != "rejected" && r.ResolutionOutcome != "revise" {
			continue
		}
		relevant := assignment == "" || len(r.BlockedAssignmentIDs) == 0 || containsID(r.BlockedAssignmentIDs, assignment)
		if relevant && (containsString(r.BlockedActions, action) || r.RequestID != "" && containsString(r.BlockedActions, "request-convert") || (action == "completion" && len(r.BlockedActions) > 0)) {
			return model.Conflict("decision-pending")
		}
	}
	return nil
}
func invalidateCandidate(t *model.Task) {
	t.CurrentCandidateID = ""
	if t.Phase == "done" || (t.Phase == "review" && t.LastCompletionID != "") {
		t.Phase = "review"
	} else if t.Phase == "review" || t.Phase == "integrating" {
		t.Phase = "active"
	}
}
func (s *Service) ensureRiskQuestion(tx *store.Tx, t *model.Task) error {
	if t.Risk != "high" {
		return nil
	}
	key := "system:risk:" + strconv.FormatInt(t.SpecVersion, 10)
	requests, e := decisionRequests(tx, t.ID)
	if e != nil {
		return e
	}
	for _, r := range requests {
		if r.ScopeKey == key || (strings.HasPrefix(r.ScopeKey, "system:risk:") && r.State == "resolved" && r.ResolutionOutcome == "approved" && r.ApprovedSpecVersion == t.SpecVersion) {
			return nil
		}
	}
	payload := decisionRequestPayload{TaskID: t.ID, ScopeKey: key, Question: "是否批准此高風險任務在所列範圍內開始實作？", Reason: "任務自報高風險，需要獨立擁有者決策", FactRefs: t.AuthorityRefs, Options: []decisionOption{{ID: "authorize-bounded", Label: "批准所列實作範圍", Impact: "外部動作仍需各自授權", Outcome: "approved"}, {ID: "revise-first", Label: "先修訂範圍", Impact: "維持阻塞", Outcome: "revise"}}, Recommendation: "revise-first", RecommendationReason: "先確認具體影響與恢復方式", Impact: t.Goal, BlockedAssignmentIDs: []model.ID{}, BlockedActions: []string{"implementation", "delivery", "integration", "acceptance", "completion", "spec-change"}, CanContinue: []string{"唯讀調查與提出修訂"}, NoDecisionBehavior: "維持相應實作動作阻塞"}
	id, version, _, e := s.requestDecision(tx, model.Actor{ID: "system", Kind: "system"}, payload)
	if e != nil {
		return e
	}
	if _, e = tx.AppendEvent(string(t.ProjectID), model.JSON(map[string]any{"kind": "decision.request", "resourceId": id, "version": version, "actorKind": "system"})); e != nil {
		return e
	}
	fresh, e := loadTask(tx, t.ID)
	if e != nil {
		return e
	}
	*t = fresh
	return nil
}
