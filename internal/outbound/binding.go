package outbound

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/store"
	"strings"
)

type Binding struct {
	ChannelID     string   `json:"channelId"`
	ID            model.ID `json:"id"`
	ProjectID     model.ID `json:"projectId"`
	Version       int64    `json:"version"`
	Kind          string   `json:"kind"`
	Repository    string   `json:"repository"`
	RepositoryID  int64    `json:"repositoryId"`
	Private       bool     `json:"private"`
	Fields        []string `json:"fields"`
	Events        []string `json:"events"`
	CredentialRef string   `json:"credentialRef"`
	Enabled       bool     `json:"enabled"`
	NeedsRebind   bool     `json:"needsRebind"`
}

var safeRef = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)
var safeRepo = regexp.MustCompile(`^[A-Za-z0-9_-]+/[A-Za-z0-9_.-]+$`)

func Validate(b Binding) error {
	if b.ChannelID != "" {
		return model.Invalid("channel identity is service-owned")
	}
	if b.ProjectID == "" || !safeRef.MatchString(b.CredentialRef) || b.NeedsRebind {
		return model.Invalid("project and product-private credential reference required")
	}
	if b.Kind == "dingtalk" {
		if b.Repository != "" || b.RepositoryID != 0 || len(b.Fields) == 0 || len(b.Events) == 0 {
			return model.Invalid("notification fields and events required; no repository")
		}
		for _, f := range b.Fields {
			if f != "taskId" && f != "goal" && f != "summary" && f != "projectTitle" {
				return model.Invalid("unsupported notification field")
			}
		}
		for _, k := range b.Events {
			if k != "decision-pending" && k != "human-acceptance" && k != "owner-blocker" {
				return model.Invalid("unsupported notification event")
			}
		}
		return nil
	}
	if b.Kind != "github" {
		return model.Invalid("unsupported binding kind")
	}
	if !safeRepo.MatchString(b.Repository) || strings.HasSuffix(b.Repository, "/.") || strings.HasSuffix(b.Repository, "/..") || b.RepositoryID <= 0 {
		return model.Invalid("canonical repository and numeric ID required")
	}
	if len(b.Fields) == 0 || len(b.Events) == 0 {
		return model.Invalid("explicit field and event allowlists required")
	}
	for _, f := range b.Fields {
		if f != "taskId" && f != "goal" && f != "state" && f != "scope" && f != "acceptance" {
			return model.Invalid("unsupported outbound field")
		}
	}
	for _, k := range b.Events {
		switch k {
		case "task.create", "task.complete", "task.cancel", "task.reopen", "integration.record", "decision.request", "decision.resolve", "decision.supersede", "delivery.submit", "delivery.review", "acceptance.candidate", "acceptance.accept":
		default:
			return model.Invalid("unsupported outbound event")
		}
	}
	return nil
}

// ResolveCredential never reads a project or vendor credential store.
func ResolveCredential(root string) func(string) ([]byte, error) {
	return func(ref string) ([]byte, error) {
		if !safeRef.MatchString(ref) {
			return nil, model.Invalid("invalid credential reference")
		}
		b, e := privatefs.ReadFile(root, filepath.Join("credentials", ref+".json"))
		if e != nil {
			return nil, model.Invalid("private credential unavailable")
		}
		return b, nil
	}
}
func ValidatePrivateCredential(root, kind, ref string) (string, error) {
	resolve := ResolveCredential(root)
	if kind == "dingtalk" {
		c, e := ReadBot(resolve, ref)
		if e != nil {
			return "", model.Invalid("private bot credential required")
		}
		return BotChannel(c.Token), nil
	}
	raw, e := resolve(ref)
	if e != nil {
		return "", model.Invalid("private GitHub credential required")
	}
	token := strings.TrimSpace(string(raw))
	if strings.HasPrefix(token, "{") {
		var c struct {
			Kind  string `json:"kind"`
			Token string `json:"token"`
		}
		if json.Unmarshal(raw, &c) != nil || c.Kind != "" && c.Kind != "github" {
			return "", model.Invalid("credential kind mismatch")
		}
		token = c.Token
	}
	if kind != "github" || token == "" || len(token) > 512 || strings.ContainsAny(token, "\r\n\t ") {
		return "", model.Invalid("private GitHub credential required")
	}
	return "", nil
}
func List(ctx context.Context, st *store.Store, a model.Actor, kind string, project model.ID, after string, limit int) (model.View, error) {
	if a.Kind != "human" {
		return model.View{}, model.Forbidden()
	}
	data := []json.RawMessage{}
	more := false
	last := ""
	e := st.Read(ctx, func(r store.Reader) error {
		query := "SELECT id,json_set(body,'$.version',version) FROM bindings WHERE (?='' OR project_id=?) AND id>? ORDER BY id LIMIT ?"
		if kind == "outbound-jobs" {
			query = "SELECT j.id,json_set(j.body,'$.status',j.status,'$.version',j.version,'$.retryAt',j.retry_at) FROM external_jobs j JOIN bindings b ON b.id=j.binding_id WHERE (?='' OR b.project_id=?) AND j.id>? ORDER BY j.id LIMIT ?"
		}
		rows, e := r.QueryContext(ctx, query, project, project, after, limit+1)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id, body string
			if e = rows.Scan(&id, &body); e != nil {
				return e
			}
			if len(data) == limit {
				more = true
				break
			}
			last = id
			data = append(data, json.RawMessage(body))
		}
		return rows.Err()
	})
	return model.View{Data: model.JSON(data), Cursor: last, HasMore: more}, e
}
