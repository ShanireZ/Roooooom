package client

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
)

type Profile struct {
	Root          string   `json:"-"`
	Handle        string   `json:"handle"`
	ServiceURL    string   `json:"serviceUrl"`
	ServiceID     string   `json:"serviceId"`
	Generation    string   `json:"datasetGeneration"`
	ProjectID     model.ID `json:"projectId"`
	ParticipantID model.ID `json:"participantId"`
	SessionID     model.ID `json:"sessionId"`
	CredentialRef string   `json:"credentialRef"`
	Mode          string   `json:"mode"`
	Provisional   bool     `json:"provisional"`
	LastSyncedAt  string   `json:"lastSyncedAt,omitempty"`
}

func ProfileRoot(dataDir, handle string) (string, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(handle) {
		return "", model.Invalid("profile handle must be 1..64 letters, digits, hyphens or underscores")
	}
	return filepath.Join(dataDir, "clients", handle), nil
}
func LoadProfile(root string) (Profile, error) {
	raw, e := privatefs.ReadFile(root, "profile.json")
	if e != nil {
		return Profile{}, e
	}
	var p Profile
	e = json.Unmarshal(raw, &p)
	p.Root = root
	return p, e
}
func SaveProfile(p Profile) error {
	return privatefs.WriteAtomic(p.Root, "profile.json", model.JSON(p))
}
