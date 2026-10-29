package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	contract "roooooom.local/roooooom/api"
	"roooooom.local/roooooom/internal/client"
	"roooooom.local/roooooom/internal/model"
	appruntime "roooooom.local/roooooom/internal/runtime"
	"time"
)

type inputCommand struct {
	model.Mutation
	TargetID model.ID        `json:"targetId"`
	Payload  json.RawMessage `json:"payload"`
}

func catalog() map[string]any {
	var document map[string]any
	_ = json.Unmarshal(contract.Contract, &document)
	return document
}
func runClient(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	if args[0] == "help" {
		doc := catalog()
		if len(args) >= 2 && args[1] == "--json" {
			commands := doc["x-command-payloads"]
			result := map[string]any{"protocolVersion": 1, "commands": commands, "cli": []string{"connect", "sync", "wait", "read", "apply", "request", "task", "assignment", "claim", "chat", "deliver", "decision", "integration", "acceptance", "session", "integrator", "collaborator", "offline", "status", "recover", "version"}}
			result["queries"] = doc["paths"].(map[string]any)["/api/v1/queries/{kind}"].(map[string]any)["get"].(map[string]any)["parameters"].([]any)[0].(map[string]any)["schema"].(map[string]any)["enum"]
			if len(args) == 3 {
				m := commands.(map[string]any)
				entry, ok := m[args[2]]
				if !ok {
					return 2
				}
				result["command"] = args[2]
				result["payload"] = entry
				result["schemas"] = doc["components"].(map[string]any)["schemas"]
			}
			return printJSON(out, result)
		}
		fmt.Fprintln(out, "Roooooom: serve | connect | sync | wait | read KIND | apply KIND | request/task/assignment/chat/deliver/decision/... ACTION | offline list/reconcile | status\nClient flags: --data-dir PATH --profile HANDLE --json. Mutations read JSON from stdin; credentials never use arguments.")
		return 0
	}
	action := ""
	kind := ""
	rest := args[1:]
	switch args[0] {
	case "connect", "sync", "wait", "status", "recover":
		action = args[0]
	case "read", "apply", "offline", "request", "task", "assignment", "integrator", "chat", "deliver", "decision", "integration", "acceptance", "session", "collaborator":
		if len(rest) == 0 {
			return 2
		}
		sub := rest[0]
		rest = rest[1:]
		switch args[0] {
		case "read":
			action = "read"
			kind = sub
		case "apply":
			action = "apply"
			kind = sub
		case "offline":
			if sub == "resolve" {
				action = "apply"
				kind = "offline.resolve"
				break
			}
			if sub != "list" && sub != "reconcile" {
				return 2
			}
			action = "offline." + sub
		case "chat":
			if sub == "inbox" || sub == "thread" || sub == "message" {
				action = "read"
				kind = sub
			} else if sub == "send" {
				action = "apply"
				kind = "message.send"
			} else if sub == "receipt" {
				action = "apply"
				kind = "receipt.update"
			} else {
				return 2
			}
		case "deliver":
			action = "apply"
			kind = "delivery." + sub
		case "task", "request":
			if sub == "get" || sub == "list" {
				action = "read"
				if sub == "get" {
					kind = args[0]
				} else {
					kind = args[0] + "s"
				}
			} else {
				action = "apply"
				kind = args[0] + "." + sub
			}
		default:
			action = "apply"
			kind = args[0] + "." + sub
		}
	case "claim":
		action = "apply"
		kind = "assignment.claim"
	default:
		fmt.Fprintln(errOut, "unknown command; use roooooom help")
		return 2
	}
	if action == "apply" {
		commands := catalog()["x-command-payloads"].(map[string]any)
		if _, ok := commands[kind]; !ok {
			return 2
		}
	}
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(errOut)
	dataDir := flags.String("data-dir", "", "private service data root")
	handle := flags.String("profile", "", "explicit participant profile")
	project := flags.String("project", "", "explicit project ID")
	session := flags.String("session", "", "reuse exactly this session; omitted connect creates a fresh one")
	target := flags.String("target", "", "read target ID")
	cursor := flags.String("cursor", "", "explicit read cursor")
	limit := flags.Int("limit", 0, "read page limit")
	seconds := flags.Int("seconds", 30, "finite wait 1..60 seconds")
	renew := flags.Bool("renew-budget", false, "explicitly renew the 10-minute wait budget")
	recoveryFile := flags.String("credential-file", "", "explicit private owner recovery grant; recover only")
	_ = flags.Bool("json", false, "JSON output")
	if e := flags.Parse(rest); e != nil || flags.NArg() != 0 {
		return 2
	}
	if *dataDir == "" {
		root, e := appruntime.DefaultRoot()
		if e != nil {
			return report(out, e)
		}
		*dataDir = root
	}
	if *handle == "" {
		fmt.Fprintln(errOut, "--profile is required")
		return 2
	}
	if action == "connect" {
		p, e := client.Connect(ctx, *dataDir, *handle, model.ID(*project), model.ID(*session), http.DefaultTransport)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"mode": p.Mode, "profile": p.Handle, "serviceId": p.ServiceID, "datasetGeneration": p.Generation, "projectId": p.ProjectID, "participantId": p.ParticipantID, "sessionId": p.SessionID, "provisional": p.Provisional, "lastSyncedAt": p.LastSyncedAt})
	}
	root, e := client.ProfileRoot(*dataDir, *handle)
	if e != nil {
		return report(out, e)
	}
	p, e := client.LoadProfile(root)
	if e != nil {
		return report(out, model.Invalid("connect this explicit profile first"))
	}
	if action == "recover" {
		p, e = client.AdoptRecovery(ctx, p, *recoveryFile, http.DefaultTransport)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"mode": p.Mode, "participantId": p.ParticipantID, "sessionId": p.SessionID, "requiresExplicitResume": true})
	}
	if *session != "" && model.ID(*session) != p.SessionID {
		return report(out, model.Conflict("profile-session-mismatch"))
	}
	c := client.New(p, http.DefaultTransport)
	defer c.Close()
	switch action {
	case "status":
		items, e := c.Pending(ctx)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"mode": p.Mode, "profile": p.Handle, "projectId": p.ProjectID, "participantId": p.ParticipantID, "sessionId": p.SessionID, "pendingDrafts": len(items), "provisional": p.Provisional})
	case "sync":
		page, e := c.Sync(ctx)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, page)
	case "wait":
		if *seconds < 1 || *seconds > 60 {
			return report(out, model.Invalid("seconds must be 1..60"))
		}
		if *renew {
			if e = c.RenewWaitBudget(ctx); e != nil {
				return report(out, e)
			}
		}
		page, e := c.Wait(ctx, time.Duration(*seconds)*time.Second)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, page)
	case "offline.list":
		items, e := c.Pending(ctx)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"mode": p.Mode, "drafts": items, "authoritative": false})
	case "offline.reconcile":
		results, e := c.Reconcile(ctx)
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"results": results})
	case "read":
		projectID := p.ProjectID
		if *project != "" {
			projectID = model.ID(*project)
		}
		view, e := c.Query(ctx, model.Query{Kind: kind, ProjectID: projectID, TargetID: model.ID(*target), Cursor: *cursor, Limit: *limit})
		if e != nil {
			if model.Status(e) == 503 {
				cached, cacheErr := c.CachedQuery(ctx, model.Query{Kind: kind, ProjectID: projectID, TargetID: model.ID(*target), Cursor: *cursor, Limit: *limit})
				if cacheErr == nil {
					return printJSON(out, map[string]any{"result": cached.Data, "cursor": cached.Cursor, "hasMore": cached.HasMore, "connection": "offline", "stale": true, "lastSyncedAt": p.LastSyncedAt})
				}
			}
			return report(out, e)
		}
		return printJSON(out, map[string]any{"result": view.Data, "cursor": view.Cursor, "hasMore": view.HasMore, "connection": "online"})
	case "apply":
		raw, e := io.ReadAll(io.LimitReader(in, 1024*1024+1))
		if e != nil || len(raw) > 1024*1024 {
			return report(out, model.Invalid("bounded JSON input required"))
		}
		var body inputCommand
		if e = model.Decode(raw, &body); e != nil {
			return report(out, e)
		}
		if body.Payload == nil {
			body.Payload = model.JSON(struct{}{})
		}
		receipt, e := c.Do(ctx, model.Command{Kind: kind, TargetID: body.TargetID, Meta: body.Mutation, Payload: body.Payload})
		if e != nil {
			return report(out, e)
		}
		return printJSON(out, map[string]any{"mode": "online", "receipt": receipt})
	}
	return 2
}
func printJSON(out io.Writer, value any) int {
	if json.NewEncoder(out).Encode(value) != nil {
		return 1
	}
	return 0
}
func report(out io.Writer, e error) int {
	code := "operation-failed"
	status := 500
	var domain *model.DomainError
	if errors.As(e, &domain) {
		code = domain.Code
		status = domain.Status
	}
	body := map[string]any{"error": map[string]any{"code": code, "status": status}, "mode": map[bool]string{true: "offline", false: "requires-attention"}[code == "offline" || code == "storage-busy" || code == "storage-full"]}
	var draft *client.DraftError
	if errors.As(e, &draft) {
		body["draftId"] = draft.LocalID
		body["requestId"] = draft.RequestID
		body["authoritative"] = false
	}
	_ = printJSON(out, body)
	return 1
}
