package model

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
)

type ID string
type Actor struct {
	ID, ParticipantID, SessionID ID
	Kind                         string
	ProjectIDs                   []ID
}
type Mutation struct {
	ServiceID               string       `json:"serviceId"`
	Generation              string       `json:"datasetGeneration"`
	ClientRequestID         string       `json:"clientRequestId"`
	ExpectedVersion         int64        `json:"expectedVersion"`
	ClaimEpoch              int64        `json:"claimEpoch"`
	IntegratorEpoch         int64        `json:"integratorEpoch"`
	SpecVersion             int64        `json:"specVersion"`
	AcceptancePolicyVersion int64        `json:"acceptancePolicyVersion"`
	SessionID               ID           `json:"sessionId"`
	DecisionVersions        map[ID]int64 `json:"decisionVersions,omitempty"`
}
type Command struct {
	Kind     string          `json:"kind"`
	TargetID ID              `json:"targetId"`
	Meta     Mutation        `json:"meta"`
	Payload  json.RawMessage `json:"payload"`
}
type Receipt struct {
	RequestID  string          `json:"requestId"`
	ResourceID ID              `json:"resourceId"`
	Version    int64           `json:"version"`
	EventID    int64           `json:"committedEventId"`
	Result     json.RawMessage `json:"result"`
}
type Query struct {
	Kind                string
	ProjectID, TargetID ID
	Cursor              string
	Limit               int
}
type View struct {
	Data    json.RawMessage
	Cursor  string
	HasMore bool
}
type DomainError struct {
	Code    string
	Status  int
	Message string
}

func (e *DomainError) Error() string { return e.Code + ": " + e.Message }
func Conflict(code string) error     { return &DomainError{code, 409, "state has changed"} }
func Invalid(message string) error   { return &DomainError{"invalid", 400, message} }
func Status(err error) int {
	var d *DomainError
	if errors.As(err, &d) {
		return d.Status
	}
	return 500
}

type Clock interface{ Now() time.Time }
type IDSource interface{ NewID() ID }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type RandomIDs struct{}

func (RandomIDs) NewID() ID {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return ID(base64.RawURLEncoding.EncodeToString(b[:]))
}
func CanonicalHash(cmd Command) (string, error) {
	var payload any
	raw := cmd.Payload
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&payload); err != nil {
		return "", Invalid("invalid JSON payload")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return "", Invalid("trailing JSON")
	}
	canonical, err := json.Marshal(struct {
		Kind     string
		TargetID ID
		Meta     Mutation
		Payload  any
	}{cmd.Kind, cmd.TargetID, cmd.Meta, payload})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
