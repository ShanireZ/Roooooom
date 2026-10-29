package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
)

func (s *Store) Write(ctx context.Context, fn func(*Tx) (model.Receipt, error)) (model.Receipt, error) {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return model.Receipt{}, Translate(err)
	}
	defer tx.Rollback()
	r, err := fn(&Tx{tx: tx, ctx: ctx})
	if err != nil {
		return model.Receipt{}, Translate(err)
	}
	if s.beforeCommit != nil {
		s.beforeCommit()
	}
	if err = tx.Commit(); err != nil {
		return model.Receipt{}, Translate(err)
	}
	if s.afterCommit != nil {
		s.afterCommit()
	}
	s.publish()
	return r, nil
}

func (s *Store) Subscribe() <-chan struct{} { s.wakeMu.Lock(); defer s.wakeMu.Unlock(); return s.wake }
func (s *Store) publish() {
	s.wakeMu.Lock()
	defer s.wakeMu.Unlock()
	close(s.wake)
	s.wake = make(chan struct{})
}
func (s *Store) CursorKey() []byte { return []byte(s.cursorKey) }
func (t *Tx) Exec(query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(t.ctx, query, args...)
}
func (t *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(t.ctx, query, args...)
}
func (t *Tx) QueryRow(query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(t.ctx, query, args...)
}
func (t *Tx) CompareAndSwap(table string, id model.ID, expected int64, body json.RawMessage) (int64, error) {
	switch table {
	case "projects", "participants", "sessions", "tasks", "assignments", "decision_requests", "bindings":
	default:
		return 0, model.Invalid("invalid record kind")
	}
	res, err := t.Exec("UPDATE "+table+" SET version=version+1,body=? WHERE id=? AND version=?", string(body), id, expected)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, model.Conflict("stale")
	}
	return expected + 1, nil
}
