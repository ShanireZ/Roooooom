package identity

import (
	"context"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
)

func (s *Service) Control(ctx context.Context, a model.Actor, c model.Command) (model.Receipt, error) {
	if a.Kind != "human" {
		return model.Receipt{}, model.Forbidden()
	}
	identity := s.store.Identity()
	if c.Meta.ServiceID == "" || c.Meta.Generation == "" || c.Meta.ClientRequestID == "" || c.TargetID == "" {
		return model.Receipt{}, model.Invalid("identity, generation, request and target required")
	}
	if c.Meta.ServiceID != identity.ServiceID || c.Meta.Generation != identity.Generation {
		return model.Receipt{}, model.Conflict("needs-reconcile")
	}
	var payload struct{}
	if e := model.Decode(c.Payload, &payload); e != nil {
		return model.Receipt{}, e
	}
	c.Payload = model.JSON(payload)
	digest, e := model.CanonicalHash(c)
	if e != nil {
		return model.Receipt{}, e
	}
	return s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		prior, e := tx.FindReceipt(a.ID, "commands", c.Meta.ClientRequestID, digest)
		if e != nil {
			return model.Receipt{}, e
		}
		if prior != nil {
			return *prior, nil
		}
		result := map[string]any{"id": c.TargetID}
		switch c.Kind {
		case "participant.recover":
			var project model.ID
			var n int
			if e := tx.QueryRow("SELECT count(*),COALESCE(min(project_id),'') FROM participant_projects WHERE participant_id=?", c.TargetID).Scan(&n, &project); e != nil {
				return model.Receipt{}, e
			}
			if n != 1 {
				return model.Receipt{}, model.Conflict("participant-not-enrolled")
			}
			credentials := Credentials{ParticipantID: c.TargetID, SessionID: model.ID(random()), Token: random()}
			if _, e := tx.Exec("INSERT INTO sessions VALUES(?,?,1,'active',?)", credentials.SessionID, c.TargetID, marshal(map[string]any{"projectId": project, "lastCheckpointAt": s.clock.Now(), "ownerRecovered": true})); e != nil {
				return model.Receipt{}, e
			}
			if e := insertToken(tx, credentials.Token, string(c.TargetID), "agent", credentials.SessionID, project, tokenBody{ParticipantID: c.TargetID}); e != nil {
				return model.Receipt{}, e
			}
			file := "recovery-credentials/" + random() + ".json"
			identity := s.store.Identity()
			if e := privatefs.WriteAtomic(s.Root(), file, model.JSON(map[string]any{"participantId": c.TargetID, "sessionId": credentials.SessionID, "token": credentials.Token, "projectId": project, "serviceId": identity.ServiceID, "datasetGeneration": identity.Generation})); e != nil {
				return model.Receipt{}, e
			}
			result["credentialPath"] = filepath.Join(s.Root(), file)
		case "enrollment.enable":
			var count int
			if e := tx.QueryRow("SELECT count(*) FROM projects WHERE id=?", c.TargetID).Scan(&count); e != nil {
				return model.Receipt{}, e
			}
			if count != 1 {
				return model.Receipt{}, model.NotFound()
			}
			cap := random()
			if _, e := tx.Exec("UPDATE tokens SET revoked=1 WHERE kind='enrollment' AND project_id=?", c.TargetID); e != nil {
				return model.Receipt{}, e
			}
			if e := insertToken(tx, cap, random(), "enrollment", nil, c.TargetID, tokenBody{}); e != nil {
				return model.Receipt{}, e
			}
			if e := privatefs.WriteAtomic(s.Root(), "enrollment/"+hash(string(c.TargetID))+".json", model.JSON(map[string]any{"projectId": c.TargetID, "capability": cap})); e != nil {
				return model.Receipt{}, e
			}
			result["enrollmentPath"] = s.EnrollmentPath(c.TargetID)
		case "participant.revoke":
			var count int
			if e := tx.QueryRow("SELECT count(*) FROM participants WHERE id=?", c.TargetID).Scan(&count); e != nil {
				return model.Receipt{}, e
			}
			if count != 1 {
				return model.Receipt{}, model.NotFound()
			}
			if _, e := tx.Exec("UPDATE tokens SET revoked=1 WHERE actor_id=? OR (kind='enrollment' AND project_id IN(SELECT project_id FROM participant_projects WHERE participant_id=?))", c.TargetID, c.TargetID); e != nil {
				return model.Receipt{}, e
			}
			if _, e := tx.Exec("DELETE FROM participant_projects WHERE participant_id=?", c.TargetID); e != nil {
				return model.Receipt{}, e
			}
		default:
			return model.Receipt{}, model.Invalid("unknown control command")
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": c.Kind, "resourceId": c.TargetID}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: c.Meta.ClientRequestID, ResourceID: c.TargetID, Version: 1, EventID: seq, Result: model.JSON(result)}
		if e = tx.SaveReceipt(a.ID, "commands", c.Meta.ClientRequestID, digest, r); e != nil {
			return model.Receipt{}, e
		}
		return r, nil
	})
}
