package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"time"
)

func Export(ctx context.Context, st *store.Store, target string) error {
	tables := []string{"projects", "repositories", "participants", "participant_projects", "sessions", "work_requests", "tasks", "assignments", "memberships", "messages", "message_receipts", "decision_requests", "decisions", "deliveries", "delivery_reviews", "integrations", "acceptance_candidates", "acceptances", "completions", "events", "request_receipts", "handoffs", "offline_candidates"}
	return privatefs.WriteStream(target, "data.json", func(out io.Writer) error {
		return st.Read(ctx, func(reader store.Reader) error {
			encoder := json.NewEncoder(out)
			if _, e := fmt.Fprintf(out, "{\"format\":\"roooooom-readable-export\",\"schemaVersion\":%d,\"createdAt\":%q,\"tables\":{", store.SchemaVersion, time.Now().UTC().Format(time.RFC3339Nano)); e != nil {
				return e
			}
			for index, table := range tables {
				if index > 0 {
					if _, e := io.WriteString(out, ","); e != nil {
						return e
					}
				}
				if _, e := fmt.Fprintf(out, "%q:[", table); e != nil {
					return e
				}
				// Table names are a fixed allowlist; no SQL is accepted from callers.
				rows, e := reader.QueryContext(ctx, "SELECT * FROM "+table)
				if e != nil {
					return e
				}
				columns, e := rows.Columns()
				if e != nil {
					rows.Close()
					return e
				}
				first := true
				for rows.Next() {
					values := make([]any, len(columns))
					pointers := make([]any, len(columns))
					for i := range pointers {
						pointers[i] = &values[i]
					}
					if e = rows.Scan(pointers...); e != nil {
						rows.Close()
						return e
					}
					record := map[string]any{}
					for i, name := range columns {
						if name == "body" || name == "result" {
							if raw, ok := values[i].(string); ok && json.Valid([]byte(raw)) {
								record[name] = json.RawMessage(raw)
								continue
							}
						}
						record[name] = values[i]
					}
					if !first {
						if _, e = io.WriteString(out, ","); e != nil {
							rows.Close()
							return e
						}
					}
					first = false
					if e = encoder.Encode(record); e != nil {
						rows.Close()
						return e
					}
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					return e
				}
				if _, e = io.WriteString(out, "]"); e != nil {
					return e
				}
			}
			_, e := io.WriteString(out, "}}\n")
			return e
		})
	})
}
