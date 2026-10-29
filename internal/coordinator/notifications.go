package coordinator

import (
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/outbound"
	"roooooom.local/roooooom/internal/store"
	"strings"
)

func (s *Service) enqueueNotifications(tx *store.Tx, c model.Command, resource model.ID, p any) error {
	var taskID model.ID
	switch {
	case c.Kind == "task.create":
		taskID = resource
	case c.Kind == "decision.request":
		if request := p.(*decisionRequestPayload).RequestID; request != "" {
			i := s.store.Identity()
			return outbound.EnqueueIntakeNotifications(tx, i.ServiceID, i.Generation, request)
		}
		taskID = p.(*decisionRequestPayload).TaskID
	case strings.HasPrefix(c.Kind, "decision."):
		r, e := loadDecisionRequest(tx, c.TargetID)
		if e != nil {
			return e
		}
		if r.RequestID != "" {
			i := s.store.Identity()
			return outbound.EnqueueIntakeNotifications(tx, i.ServiceID, i.Generation, r.RequestID)
		}
		taskID = r.TaskID
	case c.Kind == "assignment.create":
		taskID = p.(*model.AssignmentCreate).TaskID
	case strings.HasPrefix(c.Kind, "assignment.") || strings.HasPrefix(c.Kind, "delivery."):
		r, e := loadAssignment(tx, c.TargetID)
		if e != nil {
			return e
		}
		taskID = r.TaskID
	case strings.HasPrefix(c.Kind, "task.") || strings.HasPrefix(c.Kind, "integration.") || strings.HasPrefix(c.Kind, "acceptance.") || strings.HasPrefix(c.Kind, "integrator.") || c.Kind == "collaborator.limit":
		taskID = c.TargetID
	default:
		return nil
	}
	i := s.store.Identity()
	return outbound.EnqueueNotifications(tx, i.ServiceID, i.Generation, taskID)
}
