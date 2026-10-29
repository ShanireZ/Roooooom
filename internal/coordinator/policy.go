package coordinator

import (
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

type policyPayload struct {
	Policy model.AcceptancePolicy `json:"policy"`
	Reason string                 `json:"reason"`
}

func (s *Service) initializePolicy(t *model.Task, a model.Actor) {
	source := "system"
	if t.AcceptancePolicy.Mode != "integrator" || t.AcceptancePolicy.OwnerProtected {
		source = a.Kind
		if source == "human" {
			source = "owner"
		}
	}
	t.AcceptanceConstraints = model.AcceptanceConstraints{ModeSource: source, AcceptorSource: source, Requirements: []model.PolicyRequirement{}}
	for _, text := range t.AcceptancePolicy.Conditions {
		source := a.Kind
		if source == "human" {
			source = "owner"
		}
		t.AcceptanceConstraints.Requirements = append(t.AcceptanceConstraints.Requirements, model.PolicyRequirement{ID: s.ids.NewID(), Text: text, Source: source})
	}
	t.AcceptancePolicy.OwnerProtected = ownerConstraints(t.AcceptanceConstraints)
}
func ownerConstraints(c model.AcceptanceConstraints) bool {
	if c.ModeSource == "owner" || c.AcceptorSource == "owner" {
		return true
	}
	for _, r := range c.Requirements {
		if r.Source == "owner" {
			return true
		}
	}
	return false
}
func validatePolicy(tx rowReader, project model.ID, p model.AcceptancePolicy) error {
	if !validStrings(p.Conditions, false) {
		return model.Invalid("acceptance conditions required")
	}
	seen := map[string]bool{}
	for _, condition := range p.Conditions {
		if seen[condition] {
			return model.Invalid("duplicate acceptance condition")
		}
		seen[condition] = true
	}
	switch p.Mode {
	case "integrator", "human":
		if p.AcceptorID != "" {
			return model.Invalid("acceptor ID is only for designated-agent")
		}
	case "designated-agent":
		var count int
		if e := tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", p.AcceptorID, project).Scan(&count); e != nil {
			return e
		}
		if count != 1 {
			return model.Invalid("acceptor not in project")
		}
	default:
		return model.Invalid("invalid acceptance mode")
	}
	return nil
}
func (s *Service) changePolicy(tx *store.Tx, a model.Actor, c model.Command, p policyPayload) (model.ID, int64, []byte, error) {
	task, e := loadTask(tx, c.TargetID)
	if e != nil {
		return "", 0, nil, e
	}
	if e = manager(a, task, c.Meta); e != nil {
		return "", 0, nil, e
	}
	if e = version(c.Meta.ExpectedVersion, task.Version); e != nil {
		return "", 0, nil, e
	}
	if p.Reason == "" || len(p.Reason) > 8192 {
		return "", 0, nil, model.Invalid("policy change reason required")
	}
	if e = checkVersions(tx, task, "", c.Meta, true); e != nil {
		return "", 0, nil, e
	}
	if e = validatePolicy(tx, task.ProjectID, p.Policy); e != nil {
		return "", 0, nil, e
	}
	old := task.AcceptanceConstraints
	if a.Kind != "human" {
		if old.ModeSource == "owner" && p.Policy.Mode != task.AcceptancePolicy.Mode {
			return "", 0, nil, model.Forbidden()
		}
		if old.AcceptorSource == "owner" && p.Policy.AcceptorID != task.AcceptancePolicy.AcceptorID {
			return "", 0, nil, model.Forbidden()
		}
		for _, r := range old.Requirements {
			if r.Source == "owner" && !containsString(p.Policy.Conditions, r.Text) {
				return "", 0, nil, model.Forbidden()
			}
		}
	}
	requirements := []model.PolicyRequirement{}
	for _, text := range p.Policy.Conditions {
		kept := false
		for _, r := range old.Requirements {
			if r.Text == text {
				if a.Kind == "human" {
					r.Source = "owner"
				}
				requirements = append(requirements, r)
				kept = true
				break
			}
		}
		if !kept {
			source := a.Kind
			if source == "human" {
				source = "owner"
			}
			requirements = append(requirements, model.PolicyRequirement{ID: s.ids.NewID(), Text: text, Source: source})
		}
	}
	if a.Kind == "human" {
		old.ModeSource = "owner"
		old.AcceptorSource = "owner"
	} else {
		if p.Policy.Mode != task.AcceptancePolicy.Mode {
			old.ModeSource = "agent"
		}
		if p.Policy.AcceptorID != task.AcceptancePolicy.AcceptorID {
			old.AcceptorSource = "agent"
		}
	}
	old.Requirements = requirements
	task.AcceptanceConstraints = old
	p.Policy.OwnerProtected = ownerConstraints(old)
	task.AcceptancePolicy = p.Policy
	task.AcceptancePolicyVersion++
	invalidateCandidate(&task)
	if e = saveTask(tx, &task); e != nil {
		return "", 0, nil, e
	}
	return task.ID, task.Version, model.JSON(task), nil
}
