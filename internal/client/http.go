package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"strings"
	"time"
)

type Client struct {
	Profile   Profile
	transport http.RoundTripper
	clock     model.Clock
	box       *outbox
}
type ReconcileResult struct {
	LocalID     string         `json:"localId"`
	State       string         `json:"state"`
	Reason      string         `json:"reason"`
	Receipt     *model.Receipt `json:"receipt,omitempty"`
	CandidateID model.ID       `json:"candidateId,omitempty"`
}
type DraftError struct {
	LocalID, RequestID string
	Cause              error
}

func (e *DraftError) Error() string { return "operation is not confirmed; local draft retained" }
func (e *DraftError) Unwrap() error { return e.Cause }

type OutboxItem struct {
	LocalID         string        `json:"localId"`
	RequestID       string        `json:"requestId"`
	ProjectID       model.ID      `json:"projectId"`
	Command         model.Command `json:"command"`
	Cursor          string        `json:"cursor"`
	LocalOccurredAt time.Time     `json:"localOccurredAt"`
	Authoritative   bool          `json:"authoritative"`
}
type envelope struct {
	RequestID      string             `json:"requestId"`
	Result         json.RawMessage    `json:"result"`
	ObjectVersions map[model.ID]int64 `json:"objectVersions"`
	EventID        int64              `json:"committedEventId"`
	Unread         []json.RawMessage  `json:"unread"`
	Cursor         string             `json:"cursor"`
	HasMore        bool               `json:"hasMore"`
}

func New(profile Profile, transport http.RoundTripper) *Client {
	return &Client{Profile: profile, transport: transport, clock: model.RealClock{}}
}
func (c *Client) Close() error {
	if c.box != nil {
		return c.box.Close()
	}
	return nil
}
func offline() error {
	return &model.DomainError{Status: 503, Code: "offline", Message: "service unavailable; local draft retained"}
}
func (c *Client) request(ctx context.Context, method, path string, body []byte, authenticate bool) ([]byte, error) {
	base, e := url.Parse(c.Profile.ServiceURL)
	if e != nil || base.Scheme != "http" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return nil, model.Invalid("exact loopback service URL required")
	}
	host, port, e := net.SplitHostPort(base.Host)
	if e != nil || host != "127.0.0.1" || port == "" {
		return nil, model.Invalid("exact loopback service URL required")
	}
	endpoint := strings.TrimSuffix(c.Profile.ServiceURL, "/") + path
	request, e := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if e != nil {
		return nil, model.Invalid("invalid request")
	}
	request.Header.Set("Accept", "application/json")
	if method == "POST" {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticate {
		raw, e := privatefs.ReadFile(c.Profile.Root, c.Profile.CredentialRef)
		if e != nil {
			return nil, &model.DomainError{Status: 401, Code: "credentials-unavailable", Message: "owner recovery or an enrolled profile is required"}
		}
		var credentials struct{ Token string }
		if json.Unmarshal(raw, &credentials) != nil || credentials.Token == "" {
			return nil, model.Invalid("invalid private credential")
		}
		request.Header.Set("Authorization", "Bearer "+credentials.Token)
	}
	httpClient := &http.Client{Transport: c.transport, Timeout: 70 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := httpClient.Do(request)
	if e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, offline()
	}
	defer response.Body.Close()
	data, e := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if len(data) > 2*1024*1024 {
		return nil, &model.DomainError{Status: 413, Code: "response-too-large", Message: "response exceeds 2 MiB; use a narrower query or owner export; local drafts retained"}
	}
	if e != nil {
		return nil, offline()
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct{ Error struct{ Code string } }
		_ = json.Unmarshal(data, &failure)
		code := failure.Error.Code
		if code == "" {
			code = "request-rejected"
		}
		return nil, &model.DomainError{Status: response.StatusCode, Code: code, Message: "service rejected the request"}
	}
	return data, nil
}
func (c *Client) Verify(ctx context.Context) error {
	raw, e := c.request(ctx, "GET", "/identity", nil, false)
	if e != nil {
		return e
	}
	var identity struct {
		Status          string
		ServiceID       string `json:"serviceId"`
		Generation      string `json:"datasetGeneration"`
		ProtocolVersion int    `json:"protocolVersion"`
	}
	if json.Unmarshal(raw, &identity) != nil || identity.Status != "ok" || identity.ProtocolVersion != 1 || identity.ServiceID == "" || identity.Generation == "" {
		return model.Conflict("service-mismatch")
	}
	if c.Profile.ServiceID == "" || c.Profile.Generation == "" {
		return &model.DomainError{Status: 409, Code: "enrollment-required", Message: "connect with an explicitly enrolled project first"}
	}
	if identity.ServiceID != c.Profile.ServiceID || identity.Generation != c.Profile.Generation {
		return model.Conflict("needs-reconcile")
	}
	return nil
}
func (c *Client) meta(m model.Mutation) model.Mutation {
	if m.ServiceID == "" {
		m.ServiceID = c.Profile.ServiceID
	}
	if m.Generation == "" {
		m.Generation = c.Profile.Generation
	}
	if m.SessionID == "" {
		m.SessionID = c.Profile.SessionID
	}
	if m.ClientRequestID == "" {
		m.ClientRequestID = string((model.RandomIDs{}).NewID())
	}
	return m
}
func commandBody(cmd model.Command) []byte {
	return model.JSON(struct {
		model.Mutation
		TargetID model.ID        `json:"targetId,omitempty"`
		Payload  json.RawMessage `json:"payload"`
	}{cmd.Meta, cmd.TargetID, cmd.Payload})
}
func (c *Client) send(ctx context.Context, cmd model.Command) (model.Receipt, error) {
	raw, e := c.request(ctx, "POST", "/api/v1/commands/"+url.PathEscape(cmd.Kind), commandBody(cmd), true)
	if e != nil {
		return model.Receipt{}, e
	}
	var result envelope
	if e = json.Unmarshal(raw, &result); e != nil || result.RequestID != cmd.Meta.ClientRequestID || len(result.Result) == 0 {
		return model.Receipt{}, &model.DomainError{Status: 503, Code: "response-invalid", Message: "outcome unknown; original request retained"}
	}
	receipt := model.Receipt{RequestID: result.RequestID, EventID: result.EventID, Result: result.Result}
	for id, version := range result.ObjectVersions {
		receipt.ResourceID = id
		receipt.Version = version
		break
	}
	return receipt, nil
}
func (c *Client) Do(ctx context.Context, cmd model.Command) (model.Receipt, error) {
	if e := coordinator.ValidateCommandSyntax(cmd); e != nil {
		return model.Receipt{}, e
	}
	cmd.Meta = c.meta(cmd.Meta)
	cursor, e := c.cache(ctx, "cursor")
	if e != nil {
		return model.Receipt{}, e
	}
	item := OutboxItem{LocalID: string((model.RandomIDs{}).NewID()), RequestID: cmd.Meta.ClientRequestID, ProjectID: c.Profile.ProjectID, Command: cmd, Cursor: cursor, LocalOccurredAt: c.clock.Now(), Authoritative: false}
	if item, e = c.put(ctx, item); e != nil {
		return model.Receipt{}, e
	}
	if c.Profile.Mode == "offline" || c.Profile.Provisional {
		return model.Receipt{}, &DraftError{item.LocalID, item.RequestID, offline()}
	}
	if e = c.Verify(ctx); e != nil {
		return model.Receipt{}, &DraftError{item.LocalID, item.RequestID, e}
	}
	receipt, e := c.send(ctx, cmd)
	if e != nil {
		return model.Receipt{}, &DraftError{item.LocalID, item.RequestID, e}
	}
	if e = c.remove(ctx, item.LocalID); e != nil {
		return model.Receipt{}, e
	}
	return receipt, nil
}
func (c *Client) query(ctx context.Context, path string) (envelope, error) {
	if e := c.Verify(ctx); e != nil {
		return envelope{}, e
	}
	raw, e := c.request(ctx, "GET", path, nil, true)
	if e != nil {
		return envelope{}, e
	}
	var result envelope
	e = json.Unmarshal(raw, &result)
	if e != nil {
		return result, offline()
	}
	return result, nil
}
func (c *Client) syncPage(ctx context.Context, cursor string) (coordinator.SyncPage, error) {
	response, e := c.query(ctx, "/api/v1/sync?cursor="+url.QueryEscape(cursor))
	if e != nil {
		return coordinator.SyncPage{}, e
	}
	var page coordinator.SyncPage
	if e = json.Unmarshal(response.Result, &page); e != nil {
		return page, offline()
	}
	if e = c.acceptPage(ctx, page, response.Result); e != nil {
		return page, e
	}

	return page, nil
}
func (c *Client) Sync(ctx context.Context) (coordinator.SyncPage, error) {
	cursor, e := c.cache(ctx, "cursor")
	if e != nil {
		return coordinator.SyncPage{}, e
	}
	return c.syncPage(ctx, cursor)
}
func (c *Client) Reconcile(ctx context.Context) ([]ReconcileResult, error) {
	if e := c.Verify(ctx); e != nil {
		return nil, e
	}
	if e := c.snapshotCurrent(ctx); e != nil {
		return nil, e
	}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages >= 1000 {
			return nil, model.Conflict("sync-budget-exhausted")
		}
		page, e := c.syncPage(ctx, cursor)
		if e != nil {
			return nil, e
		}
		cursor = page.Cursor
		if !page.HasMore {
			break
		}
	}
	items, e := c.Pending(ctx)
	if e != nil {
		return nil, e
	}
	results := []ReconcileResult{}
	for _, item := range items {
		if e := coordinator.ValidateCommandSyntax(item.Command); e != nil {
			results = append(results, ReconcileResult{LocalID: item.LocalID, State: "rejected", Reason: "invalid command syntax; original draft retained locally; correct and submit with a new request ID"})
			continue
		}
		payload := model.JSON(map[string]any{"projectId": c.Profile.ProjectID, "localId": item.LocalID, "originalRequestId": item.RequestID, "originalCommand": item.Command, "baseCursor": item.Cursor, "localOccurredAt": item.LocalOccurredAt})
		cmd := model.Command{Kind: "offline.reconcile", Meta: c.meta(model.Mutation{ClientRequestID: "reconcile:" + item.LocalID}), Payload: payload}
		receipt, e := c.send(ctx, cmd)
		if e != nil {
			if model.Status(e) == 400 {
				results = append(results, ReconcileResult{LocalID: item.LocalID, State: "rejected", Reason: "service rejected this draft; original retained locally; no replay confirmed"})
				continue
			}
			return results, e
		}
		var result ReconcileResult
		if e = json.Unmarshal(receipt.Result, &result); e != nil {
			return results, e
		}
		results = append(results, result)
		if result.State == "committed" || result.State == "candidate" || result.State == "recorded-fact" {
			if e = c.remove(ctx, item.LocalID); e != nil {
				return results, e
			}
		}
	}
	c.Profile.Mode = "online"
	for _, result := range results {
		if result.State == "candidate" || result.State == "rejected" {
			c.Profile.Mode = "reconciling"
		}
	}
	if e = SaveProfile(c.Profile); e != nil {
		return results, e
	}
	return results, nil
}

func (c *Client) acceptPage(ctx context.Context, page coordinator.SyncPage, raw json.RawMessage) error {
	if e := c.saveCache(ctx, "snapshot", string(raw)); e != nil {
		return e
	}
	if e := c.saveCache(ctx, "cursor", page.Cursor); e != nil {
		return e
	}
	c.Profile.LastSyncedAt = c.clock.Now().Format(time.RFC3339Nano)
	if c.Profile.Mode == "offline" {
		c.Profile.Mode = "reconciling"
	}
	if e := SaveProfile(c.Profile); e != nil {
		return e
	}
	// Only a completely decoded message body may trigger fetched. References never do.
	for _, event := range page.Events {
		var body struct {
			Message *struct {
				ID           model.ID
				RecipientIDs []model.ID `json:"recipientIds"`
				Body         *string
			}
		}
		if json.Unmarshal(event.Body, &body) != nil || body.Message == nil || body.Message.Body == nil {
			continue
		}
		recipient := false
		for _, id := range body.Message.RecipientIDs {
			if id == c.Profile.ParticipantID {
				recipient = true
			}
		}
		if !recipient {
			continue
		}
		meta := c.meta(model.Mutation{ExpectedVersion: 1, ClientRequestID: "fetched:" + string(body.Message.ID) + ":" + string(c.Profile.SessionID)})
		_, _ = c.Do(ctx, model.Command{Kind: "receipt.update", TargetID: body.Message.ID, Meta: meta, Payload: model.JSON(map[string]string{"state": "fetched"})})
	}
	return nil
}
