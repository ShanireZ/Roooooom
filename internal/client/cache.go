package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
)

func queryCacheKey(q model.Query) string {
	sum := sha256.Sum256(model.JSON(q))
	return "query:" + hex.EncodeToString(sum[:])
}
func (c *Client) CachedQuery(ctx context.Context, q model.Query) (model.View, error) {
	raw, e := c.cache(ctx, queryCacheKey(q))
	if e != nil {
		return model.View{}, e
	}
	if raw == "" {
		return model.View{}, model.NotFound()
	}
	var view model.View
	e = json.Unmarshal([]byte(raw), &view)
	return view, e
}
func (c *Client) snapshotCurrent(ctx context.Context) error {
	tasks := []model.Task{}
	for _, kind := range []string{"projects", "tasks", "decisions", "inbox"} {
		cursor := ""
		for page := 0; ; page++ {
			if page >= 1000 {
				return model.Conflict("snapshot-budget-exhausted")
			}
			view, e := c.Query(ctx, model.Query{Kind: kind, ProjectID: c.Profile.ProjectID, Cursor: cursor, Limit: 100})
			if e != nil {
				return e
			}
			if kind == "tasks" {
				var batch []model.Task
				if e = json.Unmarshal(view.Data, &batch); e != nil {
					return e
				}
				tasks = append(tasks, batch...)
			}
			if !view.HasMore {
				break
			}
			cursor = view.Cursor
		}
	}
	for _, task := range tasks {
		cursor := ""
		for page := 0; ; page++ {
			if page >= 1000 {
				return model.Conflict("snapshot-budget-exhausted")
			}
			view, e := c.Query(ctx, model.Query{Kind: "assignments", ProjectID: c.Profile.ProjectID, TargetID: task.ID, Cursor: cursor, Limit: 100})
			if e != nil {
				return e
			}
			if !view.HasMore {
				break
			}
			cursor = view.Cursor
		}
	}
	return nil
}
