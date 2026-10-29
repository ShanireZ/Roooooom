package store

import (
	"context"
	"database/sql"
	"errors"
)

// The owner language lives in the meta key-value table, so it needs no DDL.
const ownerLanguageKey = "owner_language"

// OwnerLanguage returns the owner's chosen interface language, or "" when unset.
func (s *Store) OwnerLanguage(ctx context.Context) (string, error) {
	return scanLanguage(s.QueryRow(ctx, "SELECT value FROM meta WHERE key=?", ownerLanguageKey))
}

// OwnerLanguage reads the owner language inside an open transaction.
func (t *Tx) OwnerLanguage() (string, error) {
	return scanLanguage(t.QueryRow("SELECT value FROM meta WHERE key=?", ownerLanguageKey))
}

// SetOwnerLanguage stores the owner language; "" clears it.
func (t *Tx) SetOwnerLanguage(value string) error {
	if value == "" {
		_, e := t.Exec("DELETE FROM meta WHERE key=?", ownerLanguageKey)
		return e
	}
	_, e := t.Exec("INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", ownerLanguageKey, value)
	return e
}

func scanLanguage(row *sql.Row) (string, error) {
	var value string
	if e := row.Scan(&value); errors.Is(e, sql.ErrNoRows) {
		return "", nil
	} else if e != nil {
		return "", e
	}
	return value, nil
}
