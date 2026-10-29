package store

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"time"
)

type Event struct {
	Seq        int64           `json:"seq"`
	Scope      string          `json:"scope"`
	Body       json.RawMessage `json:"body"`
	ReceivedAt string          `json:"receivedAt"`
}

func (t *Tx) AppendEvent(scope string, body []byte) (int64, error) {
	if scope == "" {
		return 0, model.Invalid("event scope required")
	}
	result, err := t.Exec("INSERT INTO events(scope,body,received_at) VALUES(?,?,?)", scope, string(body), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
