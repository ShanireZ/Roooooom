package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"runtime"
)

type outbox struct {
	db   *sql.DB
	lock *flock.Flock
}

func openOutbox(root string) (result *outbox, err error) {
	if root == "" {
		return nil, model.Invalid("private client root required")
	}
	if err = privatefs.EnsureDir(root); err != nil {
		return nil, err
	}
	path := filepath.Join(root, "outbox.sqlite")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return nil, privatefs.ErrUnsafePath
	}
	result = &outbox{lock: flock.New(path + ".lock")}
	box := result
	locked, e := result.lock.TryLock()
	if e != nil || !locked {
		result.lock.Close()
		return nil, &model.DomainError{Status: 409, Code: "client-busy", Message: "another command is using this profile"}
	}
	defer func() {
		if err != nil {
			box.Close()
		}
	}()
	file := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		file = "/" + file
	}
	u := url.URL{Scheme: "file", Path: file}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	result.db, err = sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	result.db.SetMaxOpenConns(1)
	var app, tables int
	if err = result.db.QueryRow("PRAGMA application_id").Scan(&app); err != nil {
		return nil, err
	}
	if err = result.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		return nil, err
	}
	if app != 0x524f434c && (app != 0 || tables > 0) {
		return nil, model.Invalid("unrelated client database")
	}
	_, err = result.db.Exec(`PRAGMA journal_mode=WAL;
 CREATE TABLE IF NOT EXISTS drafts(id TEXT PRIMARY KEY,request_id TEXT NOT NULL UNIQUE,command_hash TEXT NOT NULL,body TEXT NOT NULL CHECK(json_valid(body)));
 CREATE TABLE IF NOT EXISTS cache(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 PRAGMA application_id=0x524f434c;PRAGMA user_version=1;`)
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (o *outbox) Close() error {
	var e error
	if o.db != nil {
		e = o.db.Close()
	}
	if o.lock != nil {
		e = errors.Join(e, o.lock.Close())
	}
	return e
}
func (c *Client) storage() (*outbox, error) {
	if c.box != nil {
		return c.box, nil
	}
	box, e := openOutbox(c.Profile.Root)
	if e != nil {
		return nil, e
	}
	c.box = box
	return box, nil
}
func (c *Client) put(ctx context.Context, item OutboxItem) (OutboxItem, error) {
	box, e := c.storage()
	if e != nil {
		return item, e
	}
	hash, e := model.CanonicalHash(item.Command)
	if e != nil {
		return item, e
	}
	var priorHash, raw string
	e = box.db.QueryRowContext(ctx, "SELECT command_hash,body FROM drafts WHERE request_id=?", item.RequestID).Scan(&priorHash, &raw)
	if e == nil {
		if priorHash != hash {
			return item, model.Conflict("request-payload-changed")
		}
		e = json.Unmarshal([]byte(raw), &item)
		return item, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return item, e
	}
	_, e = box.db.ExecContext(ctx, "INSERT INTO drafts VALUES(?,?,?,?)", item.LocalID, item.RequestID, hash, string(model.JSON(item)))
	return item, e
}
func (c *Client) Pending(ctx context.Context) ([]OutboxItem, error) {
	box, e := c.storage()
	if e != nil {
		return nil, e
	}
	rows, e := box.db.QueryContext(ctx, "SELECT body FROM drafts ORDER BY rowid")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []OutboxItem{}
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var item OutboxItem
		if e = json.Unmarshal([]byte(raw), &item); e != nil {
			return nil, e
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (c *Client) remove(ctx context.Context, id string) error {
	box, e := c.storage()
	if e != nil {
		return e
	}
	_, e = box.db.ExecContext(ctx, "DELETE FROM drafts WHERE id=?", id)
	return e
}
func (c *Client) cache(ctx context.Context, key string) (string, error) {
	box, e := c.storage()
	if e != nil {
		return "", e
	}
	var value string
	e = box.db.QueryRowContext(ctx, "SELECT value FROM cache WHERE key=?", key).Scan(&value)
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil
	}
	return value, e
}
func (c *Client) saveCache(ctx context.Context, key, value string) error {
	box, e := c.storage()
	if e != nil {
		return e
	}
	_, e = box.db.ExecContext(ctx, "INSERT INTO cache VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return e
}
