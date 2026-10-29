package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"roooooom.local/roooooom/internal/model"
)

func (t *Tx) FindReceipt(actor model.ID, scope, key, hash string) (*model.Receipt, error) {
	var stored string
	var body []byte
	err := t.QueryRow("SELECT content_hash,result FROM request_receipts WHERE actor_id=? AND scope=? AND request_key=?", actor, scope, key).Scan(&stored, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stored != hash {
		return nil, model.Conflict("request_key_conflict")
	}
	var r model.Receipt
	if err = json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
func (t *Tx) SaveReceipt(actor model.ID, scope, key, hash string, r model.Receipt) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = t.Exec("INSERT INTO request_receipts(actor_id,scope,request_key,content_hash,event_id,result) VALUES(?,?,?,?,?,?)", actor, scope, key, hash, r.EventID, string(data))
	return err
}
