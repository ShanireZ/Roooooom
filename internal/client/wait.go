package client

import (
	"context"
	"encoding/json"
	"net/url"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/model"
	"strconv"
	"time"
)

func (c *Client) Wait(ctx context.Context, d time.Duration) (coordinator.SyncPage, error) {
	if d == 0 {
		d = 30 * time.Second
	}
	if d < 0 || d > time.Minute {
		return coordinator.SyncPage{}, model.Invalid("wait duration must be at most 60 seconds")
	}
	start, e := c.cache(ctx, "wait-start")
	if e != nil {
		return coordinator.SyncPage{}, e
	}
	now := c.clock.Now()
	started := now
	if start == "" {
		if e = c.saveCache(ctx, "wait-start", now.Format(time.RFC3339Nano)); e != nil {
			return coordinator.SyncPage{}, e
		}
	} else {
		started, e = time.Parse(time.RFC3339Nano, start)
		if e != nil {
			return coordinator.SyncPage{}, e
		}
	}
	remaining := 10*time.Minute - now.Sub(started)
	if remaining <= 0 {
		return coordinator.SyncPage{}, &model.DomainError{Status: 409, Code: "wait-budget-exhausted", Message: "waiting; no agent is receiving now; explicitly renew to wait again"}
	}
	if d > remaining {
		d = remaining
	}
	budget := d + time.Second
	if budget > remaining {
		budget = remaining
	}
	bounded, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	cursor, e := c.cache(ctx, "cursor")
	if e != nil {
		return coordinator.SyncPage{}, e
	}
	seconds := int((d + time.Second - 1) / time.Second)
	response, e := c.query(bounded, "/api/v1/wait?cursor="+url.QueryEscape(cursor)+"&timeoutSeconds="+strconv.Itoa(seconds))
	if e != nil {
		return coordinator.SyncPage{}, e
	}
	var page coordinator.SyncPage
	if e = json.Unmarshal(response.Result, &page); e != nil {
		return page, e
	}
	if e = c.acceptPage(ctx, page, response.Result); e != nil {
		return page, e
	}
	if len(page.Events) > 0 {
		_ = c.saveCache(ctx, "wait-start", "")
	}
	return page, nil
}
func (c *Client) RenewWaitBudget(ctx context.Context) error {
	return c.saveCache(ctx, "wait-start", "")
}
