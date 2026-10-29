package identity

import (
	"context"
	"encoding/json"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
)

func (s *Service) EnrollmentPath(project model.ID) string {
	return filepath.Join(s.Root(), "enrollment", hash(string(project))+".json")
}
func (s *Service) EnableEnrollment(ctx context.Context, actor model.Actor, project model.ID) (string, error) {
	_, e := s.Control(ctx, actor, s.controlCommand("enrollment.enable", project))
	if e != nil {
		return "", e
	}
	raw, e := privatefs.ReadFile(s.Root(), "enrollment/"+hash(string(project))+".json")
	if e != nil {
		return "", e
	}
	var data struct{ Capability string }
	e = json.Unmarshal(raw, &data)
	return data.Capability, e
}
func (s *Service) Enroll(ctx context.Context, cap, requestID, name string) (Credentials, error) {
	a, e := s.Authenticate(ctx, cap)
	if e != nil {
		return Credentials{}, e
	}
	if a.Kind != "enrollment" {
		return Credentials{}, denied(403, "forbidden")
	}
	if requestID == "" || len(requestID) > 128 || name == "" || len(name) > 256 {
		return Credentials{}, model.Invalid("request ID and display name required")
	}
	project := a.ProjectIDs[0]
	digest, e := model.CanonicalHash(model.Command{Kind: "enroll", Payload: json.RawMessage(marshal(map[string]string{"name": name}))})
	if e != nil {
		return Credentials{}, e
	}
	receipt, e := s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var revoked int
		if e := tx.QueryRow("SELECT revoked FROM tokens WHERE hash=?", hash(cap)).Scan(&revoked); e != nil || revoked != 0 {
			return model.Receipt{}, denied(401, "unauthorized")
		}
		prior, e := tx.FindReceipt(a.ID, string(project), requestID, digest)
		if e != nil {
			return model.Receipt{}, e
		}
		if prior != nil {
			return *prior, nil
		}
		credentials := Credentials{model.ID(random()), model.ID(random()), random()}
		file := "credentials/" + random() + ".json"
		if _, e := tx.Exec("INSERT INTO participants VALUES(?,1,?)", credentials.ParticipantID, marshal(map[string]string{"name": name})); e != nil {
			return model.Receipt{}, e
		}
		if _, e := tx.Exec("INSERT INTO participant_projects VALUES(?,?)", credentials.ParticipantID, project); e != nil {
			return model.Receipt{}, e
		}
		if _, e := tx.Exec("INSERT INTO sessions VALUES(?,?,1,'active',?)", credentials.SessionID, credentials.ParticipantID, string(model.JSON(map[string]any{"projectId": project, "lastCheckpointAt": s.clock.Now()}))); e != nil {
			return model.Receipt{}, e
		}
		if e := insertToken(tx, credentials.Token, string(credentials.ParticipantID), "agent", credentials.SessionID, project, tokenBody{ParticipantID: credentials.ParticipantID}); e != nil {
			return model.Receipt{}, e
		}
		if e := privatefs.WriteAtomic(s.Root(), file, []byte(marshal(credentials))); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent(string(project), []byte(marshal(map[string]any{"kind": "agent.enrolled", "participantId": credentials.ParticipantID, "sessionId": credentials.SessionID})))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: requestID, ResourceID: credentials.ParticipantID, Version: 1, EventID: seq, Result: json.RawMessage(marshal(map[string]string{"credentialFile": file}))}
		if e := tx.SaveReceipt(a.ID, string(project), requestID, digest, r); e != nil {
			return model.Receipt{}, e
		}
		return r, nil
	})
	if e != nil {
		return Credentials{}, e
	}
	var ref struct{ CredentialFile string }
	if e = json.Unmarshal(receipt.Result, &ref); e != nil {
		return Credentials{}, e
	}
	data, e := privatefs.ReadFile(s.Root(), ref.CredentialFile)
	if e != nil {
		return Credentials{}, e
	}
	var credentials Credentials
	e = json.Unmarshal(data, &credentials)
	return credentials, e
}
func (s *Service) Revoke(ctx context.Context, actor model.Actor, participant model.ID) error {
	_, e := s.Control(ctx, actor, s.controlCommand("participant.revoke", participant))
	return e
}
func (s *Service) controlCommand(kind string, target model.ID) model.Command {
	i := s.store.Identity()
	return model.Command{Kind: kind, TargetID: target, Meta: model.Mutation{ServiceID: i.ServiceID, Generation: i.Generation, ClientRequestID: random()}, Payload: model.JSON(struct{}{})}
}
