package runtime

import (
	"context"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
)

type Limits struct {
	DatabaseBytes int64 `json:"databaseBytes"`
	SoftBytes     int64 `json:"softBytes"`
	HardBytes     int64 `json:"hardBytes"`
	Warning       bool  `json:"warning"`
}

const DefaultDatabaseLimit int64 = 1 << 30

func SetDatabaseLimit(ctx context.Context, st *store.Store, bytes int64) error {
	if bytes < 1<<20 || bytes > 1<<40 {
		return model.Invalid("database limit must be 1 MiB..1 TiB")
	}
	return st.SetPageLimit(ctx, bytes)
}
