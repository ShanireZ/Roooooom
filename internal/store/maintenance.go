package store

import (
	"context"
	"errors"
	"fmt"
	"modernc.org/sqlite"
	"roooooom.local/roooooom/internal/model"
)

func (s *Store) SetPageLimit(ctx context.Context, bytes int64) error {
	var page int64
	if e := s.writer.QueryRowContext(ctx, "PRAGMA page_size").Scan(&page); e != nil {
		return Translate(e)
	}
	var actual int64
	if e := s.writer.QueryRowContext(ctx, fmt.Sprintf("PRAGMA max_page_count=%d", bytes/page)).Scan(&actual); e != nil {
		return Translate(e)
	}
	if actual*page > bytes {
		return model.Conflict("database-already-exceeds-limit")
	}
	return nil
}

type Usage struct {
	DatabaseBytes int64 `json:"databaseBytes"`
	HardBytes     int64 `json:"hardBytes"`
	SoftBytes     int64 `json:"softBytes"`
	Warning       bool  `json:"warning"`
}

func (s *Store) Usage(ctx context.Context) (Usage, error) {
	var page, count, max int64
	for _, x := range []struct {
		query string
		value *int64
	}{{"PRAGMA page_size", &page}, {"PRAGMA page_count", &count}, {"PRAGMA max_page_count", &max}} {
		if e := s.writer.QueryRowContext(ctx, x.query).Scan(x.value); e != nil {
			return Usage{}, Translate(e)
		}
	}
	return Usage{count * page, max * page, max * page * 3 / 4, count*4 >= max*3}, nil
}

func Translate(err error) error {
	if err == nil {
		return nil
	}
	var domain *model.DomainError
	if errors.As(err, &domain) {
		return err
	}
	var engine *sqlite.Error
	if errors.As(err, &engine) {
		switch engine.Code() & 255 {
		case 13:
			return &model.DomainError{Status: 507, Code: "storage-full", Message: "storage limit reached; retain an offline draft"}
		case 5, 6:
			return &model.DomainError{Status: 503, Code: "storage-busy", Message: "storage is busy; retry the same request"}
		}
	}
	return err
}
func (s *Store) VacuumInto(ctx context.Context, path string) error {
	_, e := s.writer.ExecContext(ctx, "VACUUM INTO ?", path)
	return Translate(e)
}
func (s *Store) Checkpoint(ctx context.Context) error {
	var busy, log, done int
	e := s.writer.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &done)
	if e != nil {
		return Translate(e)
	}
	if busy != 0 {
		return &model.DomainError{Status: 503, Code: "storage-busy", Message: "active readers prevent maintenance"}
	}
	return nil
}
