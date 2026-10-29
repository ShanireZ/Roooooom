package model

import "time"

type AcceptancePolicy struct {
	Mode           string   `json:"mode"`
	AcceptorID     ID       `json:"acceptorId,omitempty"`
	Conditions     []string `json:"conditions"`
	OwnerProtected bool     `json:"ownerProtected"`
}
type TaskCreate struct {
	ProjectID              ID               `json:"projectId"`
	Goal                   string           `json:"goal"`
	Scope                  []string         `json:"scope"`
	NonScope               []string         `json:"nonScope"`
	AuthorityRefs          []string         `json:"authorityRefs"`
	Risk                   string           `json:"risk"`
	AcceptancePolicy       AcceptancePolicy `json:"acceptancePolicy"`
	CollaboratorLimit      int              `json:"collaboratorLimit"`
	DesignatedIntegratorID ID               `json:"designatedIntegratorId,omitempty"`
}
type Task struct {
	CollaboratorLimitSource string      `json:"collaboratorLimitSource,omitempty"`
	SourceRequestID         ID          `json:"sourceRequestId,omitempty"`
	SourceRequestText       string      `json:"sourceRequestText,omitempty"`
	SourceConstraints       []string    `json:"sourceConstraints,omitempty"`
	RecordRefs              []RecordRef `json:"recordRefs,omitempty"`
	ParticipantIDs          []ID        `json:"participantIds,omitempty"`
	TaskCreate
	ID                      ID                    `json:"id"`
	Version                 int64                 `json:"version"`
	SpecVersion             int64                 `json:"specVersion"`
	AcceptancePolicyVersion int64                 `json:"acceptancePolicyVersion"`
	CreatedBy               ID                    `json:"createdBy"`
	ActorKind               string                `json:"actorKind"`
	IntegratorID            ID                    `json:"integratorId,omitempty"`
	IntegratorSessionID     ID                    `json:"integratorSessionId,omitempty"`
	IntegratorEpoch         int64                 `json:"integratorEpoch"`
	Phase                   string                `json:"phase"`
	CanStart                bool                  `json:"canStart"`
	CollaboratorCount       int                   `json:"collaboratorCount"`
	Attention               []string              `json:"attention"`
	AcceptanceConstraints   AcceptanceConstraints `json:"acceptanceConstraints"`
	CurrentCandidateID      ID                    `json:"currentCandidateId,omitempty"`
	LatestIntegrationID     ID                    `json:"latestIntegrationId,omitempty"`
	LastCompletionID        ID                    `json:"lastCompletionId,omitempty"`
	DecisionVersions        map[ID]int64          `json:"decisionVersions"`
	// Read-only view times derived from events; never written to task bodies.
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type RecordRef struct {
	BindingID   ID     `json:"bindingId"`
	Number      string `json:"number"`
	URL         string `json:"url"`
	TaskVersion int64  `json:"taskVersion"`
}
type AssignmentCreate struct {
	TaskID       ID       `json:"taskId"`
	Role         string   `json:"role"`
	Input        string   `json:"input"`
	Output       string   `json:"output"`
	WriteScope   []string `json:"writeScope"`
	RepoID       ID       `json:"repoId"`
	BaseRef      string   `json:"baseRef"`
	Dependencies []ID     `json:"dependencies"`
}
type Assignment struct {
	AssignmentCreate
	ID                 ID     `json:"id"`
	Version            int64  `json:"version"`
	ClaimParticipantID ID     `json:"claimParticipantId,omitempty"`
	ClaimSessionID     ID     `json:"claimSessionId,omitempty"`
	ClaimEpoch         int64  `json:"claimEpoch"`
	State              string `json:"state"`
	Required           bool   `json:"required"`
	CanStart           bool   `json:"canStart"`
	CurrentDeliveryID  ID     `json:"currentDeliveryId,omitempty"`
	AcceptedDeliveryID ID     `json:"acceptedDeliveryId,omitempty"`
	ContractVersion    int64  `json:"contractVersion"`
	CancellationReason string `json:"cancellationReason,omitempty"`
}
type PolicyRequirement struct {
	ID     ID     `json:"id"`
	Text   string `json:"text"`
	Source string `json:"source"`
}
type AcceptanceConstraints struct {
	ModeSource     string              `json:"modeSource"`
	AcceptorSource string              `json:"acceptorSource"`
	Requirements   []PolicyRequirement `json:"requirements"`
}
