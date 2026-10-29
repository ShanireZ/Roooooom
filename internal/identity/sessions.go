package identity

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
)

func (s *Service) NewSession(ctx context.Context, a model.Actor, requestID string) (Credentials, error) {
	if a.Kind != "agent" || len(a.ProjectIDs) != 1 {
		return Credentials{}, model.Forbidden()
	}
	if requestID == "" || len(requestID) > 128 {
		return Credentials{}, model.Invalid("request ID required")
	}
	project := a.ProjectIDs[0]
	digest, e := model.CanonicalHash(model.Command{Kind: "session.new", TargetID: a.SessionID, Payload: model.JSON(map[string]any{"projectId": project})})
	if e != nil {
		return Credentials{}, e
	}
	receipt, e := s.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		var n int
		if e := tx.QueryRow("SELECT count(*) FROM participant_projects WHERE participant_id=? AND project_id=?", a.ParticipantID, project).Scan(&n); e != nil {
			return model.Receipt{}, e
		}
		if n != 1 {
			return model.Receipt{}, model.Forbidden()
		}
		prior, e := tx.FindReceipt(a.ID, "identity.sessions", requestID, digest)
		if e != nil {
			return model.Receipt{}, e
		}
		if prior != nil {
			return *prior, nil
		}
		credentials := Credentials{a.ParticipantID, model.ID(random()), random()}
		file := "credentials/" + random() + ".json"
		if _, e := tx.Exec("INSERT INTO sessions VALUES(?,?,1,'active',?)", credentials.SessionID, a.ParticipantID, string(model.JSON(map[string]any{"projectId": project, "lastCheckpointAt": s.clock.Now()}))); e != nil {
			return model.Receipt{}, e
		}
		if e := insertToken(tx, credentials.Token, string(a.ParticipantID), "agent", credentials.SessionID, project, tokenBody{ParticipantID: a.ParticipantID}); e != nil {
			return model.Receipt{}, e
		}
		if e := privatefs.WriteAtomic(s.Root(), file, model.JSON(credentials)); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent(string(project), model.JSON(map[string]any{"kind": "session.created", "participantId": a.ParticipantID, "sessionId": credentials.SessionID}))
		if e != nil {
			return model.Receipt{}, e
		}
		r := model.Receipt{RequestID: requestID, ResourceID: credentials.SessionID, Version: 1, EventID: seq, Result: model.JSON(map[string]string{"credentialFile": file})}
		if e = tx.SaveReceipt(a.ID, "identity.sessions", requestID, digest, r); e != nil {
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
	raw, e := privatefs.ReadFile(s.Root(), ref.CredentialFile)
	if e != nil {
		return Credentials{}, e
	}
	var credentials Credentials
	e = json.Unmarshal(raw, &credentials)
	return credentials, e
}
