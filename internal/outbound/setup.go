package outbound

import (
	"context"
	"encoding/json"
	"net/url"
	"roooooom.local/roooooom/internal/model"
	"strings"
)

type RepositoryInspection struct {
	Repository          string `json:"repository"`
	RepositoryID        int64  `json:"repositoryId"`
	Private             bool   `json:"private"`
	HasIssues           bool   `json:"hasIssues"`
	IssuesWriteVerified bool   `json:"issuesWriteVerified"`
}
type RepositoryInspector interface {
	Inspect(context.Context, string, string) (RepositoryInspection, error)
}

func RepositoryName(reference string) (string, error) {
	name := reference
	if strings.Contains(reference, "://") {
		u, e := url.Parse(reference)
		if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(reference,"#") || u.RawPath != "" {
			return "", model.Invalid("official GitHub repository URL required")
		}
		name = strings.TrimPrefix(u.Path, "/")
	}
	name = strings.TrimSuffix(name, ".git")
	if len(name) > 256 || !safeRepo.MatchString(name) || strings.HasSuffix(name, "/.") || strings.HasSuffix(name, "/..") {
		return "", model.Invalid("owner/repository required")
	}
	return name, nil
}

func (g *GitHub) Inspect(ctx context.Context, reference, ref string) (RepositoryInspection, error) {
	var result RepositoryInspection
	name, e := RepositoryName(reference)
	if e != nil {
		return result, e
	}
	if !safeRef.MatchString(ref) {
		return result, model.Invalid("private credential reference required")
	}
	b, response, e := g.request(ctx, Job{Binding: Binding{CredentialRef: ref}}, "GET", "/repos/"+name, nil)
	// Never return provider bodies, headers or transport URLs containing secrets.
	if e != nil {
		return result, model.Conflict("github-metadata-unavailable")
	}
	if response.StatusCode != 200 {
		return result, model.Conflict("github-metadata-denied-or-moved")
	}
	var repo struct {
		ID        int64  `json:"id"`
		Name      string `json:"full_name"`
		Private   bool   `json:"private"`
		HasIssues bool   `json:"has_issues"`
		Archived  bool   `json:"archived"`
		Disabled  bool   `json:"disabled"`
	}
	if json.Unmarshal(b, &repo) != nil || repo.ID <= 0 || !strings.EqualFold(repo.Name, name) || !repo.HasIssues || repo.Archived || repo.Disabled {
		return result, model.Conflict("github-repository-unavailable")
	}
	// Metadata access cannot establish Issues write permission. No probe write.
	return RepositoryInspection{Repository: repo.Name, RepositoryID: repo.ID, Private: repo.Private, HasIssues: repo.HasIssues}, nil
}

func ParseWebhook(reference string) (string, error) {
	invalid := model.Invalid("official signed DingTalk webhook URL required")
	u, e := url.Parse(reference)
	if e != nil || len(reference)>2048 || u.Scheme != "https" || u.Host != "oapi.dingtalk.com" || u.User != nil || u.Path != "/robot/send" || u.RawPath != "" || strings.Contains(reference,"#") {
		return "", invalid
	}
	values, e := url.ParseQuery(u.RawQuery)
	if e != nil || len(values) != 1 || len(values["access_token"]) != 1 {
		return "", invalid
	}
	token := values.Get("access_token")
	if token == "" || len(token) > 512 || strings.ContainsAny(token, "\r\n\t ") {
		return "", invalid
	}
	return token, nil
}
