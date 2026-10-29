package model

import "time"

// RequestCreate is human intent, not an executable Task contract.
type RequestCreate struct {
	ProjectID              ID                `json:"projectId"`
	Text                   string            `json:"text"`
	Constraints            []string          `json:"constraints,omitempty"`
	CollaboratorLimit      *int              `json:"collaboratorLimit,omitempty"`
	DesignatedIntegratorID ID                `json:"designatedIntegratorId,omitempty"`
	AcceptancePolicy       *AcceptancePolicy `json:"acceptancePolicy,omitempty"`
}
type WorkRequest struct {
	RequestCreate
	ID                    ID        `json:"id"`
	Version               int64     `json:"version"`
	CreatedBy             ID        `json:"createdBy"`
	ActorKind             string    `json:"actorKind"`
	CreatedAt             time.Time `json:"createdAt"`
	State                 string    `json:"state"`
	PreparerParticipantID ID        `json:"preparerParticipantId,omitempty"`
	PreparerSessionID     ID        `json:"preparerSessionId,omitempty"`
	PreparationEpoch      int64     `json:"preparationEpoch"`
	OfferedSessionID      ID        `json:"offeredSessionId,omitempty"`
	TaskID                ID        `json:"taskId,omitempty"`
	ConvertedSpecVersion  int64     `json:"convertedSpecVersion,omitempty"`
	LastReason            string    `json:"lastReason,omitempty"`
}
