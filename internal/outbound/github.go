package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type GitHub struct {
	base    string
	client  *http.Client
	resolve func(string) ([]byte, error)
}

var errPrivateCredential = errors.New("private integration credential unavailable")

func NewGitHub(resolve func(string) ([]byte, error)) *GitHub {
	return newGitHub("https://api.github.com", &http.Client{Timeout: 15 * time.Second}, resolve)
}

// NewGitHubForTest injects an isolated fake transport; production exposes no API-host setting.
func NewGitHubForTest(base string, c *http.Client, resolve func(string) ([]byte, error)) *GitHub {
	return newGitHub(base, c, resolve)
}
func newGitHub(base string, c *http.Client, resolve func(string) ([]byte, error)) *GitHub {
	copy := *c
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &GitHub{strings.TrimRight(base, "/"), &copy, resolve}
}
func (g *GitHub) request(ctx context.Context, j Job, method, path string, payload any) ([]byte, *http.Response, error) {
	secret, e := g.resolve(j.Binding.CredentialRef)
	if e != nil {
		return nil, nil, errPrivateCredential
	}
	// The owner-provided product-private file may contain a PAT directly, or a
	// JSON object. A JSON envelope is shared with the notification adapter.
	token := strings.TrimSpace(string(secret))
	if strings.HasPrefix(token, "{") {
		var v struct {
			Token string `json:"token"`
			Kind  string `json:"kind"`
		}
		if e = json.Unmarshal(secret, &v); e != nil {
			return nil, nil, errPrivateCredential
		}
		token = v.Token
		if v.Kind != "" && v.Kind != "github" {
			return nil, nil, errPrivateCredential
		}
	}
	if token == "" || strings.ContainsAny(token, "\r\n\t ") {
		return nil, nil, errPrivateCredential
	}
	req, e := http.NewRequestWithContext(ctx, method, g.base+path, bytes.NewReader(mustBytes(payload)))
	if e != nil {
		return nil, nil, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("Content-Type", "application/json")
	response, e := g.client.Do(req)
	if e != nil {
		return nil, nil, e
	}
	defer response.Body.Close()
	b, e := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if len(b) > 2*1024*1024 {
		return nil, response, io.ErrUnexpectedEOF
	}
	return b, response, e
}
func mustBytes(v any) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}
func rateResult(r *http.Response) (Result, bool) {
	next := time.Time{}
	if s := r.Header.Get("Retry-After"); s != "" {
		if n, e := strconv.Atoi(s); e == nil && n >= 0 {
			next = time.Now().Add(time.Duration(n) * time.Second)
		} else if t, e := http.ParseTime(s); e == nil {
			next = t
		}
	}
	if r.Header.Get("X-RateLimit-Remaining") == "0" {
		if n, e := strconv.ParseInt(r.Header.Get("X-RateLimit-Reset"), 10, 64); e == nil && time.Unix(n, 0).After(next) {
			next = time.Unix(n, 0)
		}
	}
	if r.StatusCode == 429 || r.StatusCode == 403 && !next.IsZero() {
		if next.IsZero() {
			next = time.Now().Add(time.Minute)
		}
		return Result{State: "retry", RetryAt: next, ErrorCode: "remote-rate-limit"}, true
	}
	return Result{}, false
}
func (g *GitHub) pin(ctx context.Context, j Job) (Result, bool) {
	if !safeRepo.MatchString(j.Binding.Repository) || j.Binding.RepositoryID <= 0 {
		return Result{State: "paused", ErrorCode: "repository-pin-invalid"}, false
	}
	b, r, e := g.request(ctx, j, "GET", "/repos/"+j.Binding.Repository, nil)
	if e != nil {
		if errors.Is(e, errPrivateCredential) {
			return Result{State: "paused", ErrorCode: "private-credential-unavailable"}, false
		}
		return Result{State: "retry", ErrorCode: "metadata-unavailable", RetryAt: time.Now().Add(time.Minute)}, false
	}
	if rate, ok := rateResult(r); ok {
		return rate, false
	}
	if r.StatusCode >= 500 {
		return Result{State: "retry", ErrorCode: "metadata-unavailable", RetryAt: time.Now().Add(time.Minute)}, false
	}
	var repo struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
	}
	if r.StatusCode != 200 || json.Unmarshal(b, &repo) != nil || repo.ID != j.Binding.RepositoryID || !strings.EqualFold(repo.FullName, j.Binding.Repository) || repo.Private != j.Binding.Private {
		return Result{State: "paused", ErrorCode: "repository-pin-mismatch"}, false
	}
	return Result{}, true
}
func (g *GitHub) Deliver(ctx context.Context, j Job) (Result, error) {
	if r, ok := g.pin(ctx, j); !ok {
		return r, nil
	}
	path := "/repos/" + j.Binding.Repository + "/issues"
	method := "POST"
	body := map[string]string{"title": j.Title, "body": j.Text + "\n" + j.Marker}
	switch j.Kind {
	case "github.issue":
	case "github.comment", "github.state":
		if n, e := strconv.ParseInt(j.IssueNumber, 10, 64); e != nil || n <= 0 {
			return Result{State: "paused", ErrorCode: "issue-reference-invalid"}, nil
		}
		path += "/" + j.IssueNumber
		if j.Kind == "github.comment" {
			path += "/comments"
			delete(body, "title")
		} else {
			method = "PATCH"
			if j.State != "open" && j.State != "closed" {
				return Result{State: "paused", ErrorCode: "invalid-projection-state"}, nil
			}
			body = map[string]string{"state": j.State}
		}
	default:
		return Result{State: "paused", ErrorCode: "unsupported-job"}, nil
	}
	b, response, e := g.request(ctx, j, method, path, body)
	if e != nil {
		return Result{State: "uncertain", ErrorCode: "remote-outcome-unknown"}, nil
	}
	if r, ok := rateResult(response); ok {
		return r, nil
	}
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return Result{State: "paused", ErrorCode: "credential-or-permission"}, nil
	}
	if response.StatusCode >= 500 {
		return Result{State: "uncertain", ErrorCode: "remote-outcome-unknown"}, nil
	}
	if response.StatusCode != 201 && response.StatusCode != 200 {
		return Result{State: "paused", ErrorCode: "remote-rejected"}, nil
	}
	var remote struct {
		Number int64  `json:"number"`
		ID     int64  `json:"id"`
		URL    string `json:"html_url"`
	}
	if json.Unmarshal(b, &remote) != nil {
		return Result{State: "uncertain", ErrorCode: "invalid-remote-response"}, nil
	}
	number := remote.Number
	if j.Kind == "github.comment" {
		number = remote.ID
	}
	if number <= 0 {
		return Result{State: "uncertain", ErrorCode: "missing-remote-reference"}, nil
	}
	urlPrefix := "https://github.com/" + j.Binding.Repository + "/issues/"
	if !strings.HasPrefix(remote.URL, urlPrefix) {
		return Result{State: "uncertain", ErrorCode: "remote-url-mismatch"}, nil
	}
	return Result{State: "delivered", RemoteID: strconv.FormatInt(number, 10), RemoteURL: githubURL(j, number)}, nil
}
func githubURL(j Job, id int64) string {
	number := strconv.FormatInt(id, 10)
	suffix := ""
	if j.Kind == "github.comment" {
		number = j.IssueNumber
		suffix = "#issuecomment-" + strconv.FormatInt(id, 10)
	}
	return "https://github.com/" + j.Binding.Repository + "/issues/" + number + suffix
}
func (g *GitHub) Reconcile(ctx context.Context, j Job) (Result, error) {
	if r, ok := g.pin(ctx, j); !ok {
		return r, nil
	}
	path := "/repos/" + j.Binding.Repository + "/issues"
	query := "?state=all&per_page=100&page="
	if j.Kind != "github.issue" {
		if n, e := strconv.ParseInt(j.IssueNumber, 10, 64); e != nil || n <= 0 {
			return Result{State: "paused", ErrorCode: "issue-reference-invalid"}, nil
		}
		path += "/" + j.IssueNumber
	}
	if j.Kind == "github.state" {
		b, r, e := g.request(ctx, j, "GET", path, nil)
		if e == nil && r.StatusCode == 200 {
			var v struct {
				State string `json:"state"`
				URL   string `json:"html_url"`
			}
			if json.Unmarshal(b, &v) == nil && v.State == j.State {
				return Result{State: "delivered", RemoteID: j.IssueNumber, RemoteURL: "https://github.com/" + j.Binding.Repository + "/issues/" + j.IssueNumber}, nil
			}
		}
		return Result{State: "uncertain", ErrorCode: "state-not-confirmed"}, nil
	}
	if j.Kind == "github.comment" {
		path += "/comments"
		query = "?per_page=100&page="
	}
	// Bounded listing avoids search-index assumptions. Absence is never proof
	// that a non-idempotent write did not happen; no automatic recreation.
	found := []Result{}
	complete := false
	for page := 1; page <= 10; page++ {
		b, r, e := g.request(ctx, j, "GET", path+query+strconv.Itoa(page), nil)
		if e != nil {
			return Result{State: "uncertain", ErrorCode: "reconciliation-unavailable"}, nil
		}
		if rate, ok := rateResult(r); ok {
			rate.State = "uncertain"
			return rate, nil
		}
		if r.StatusCode != 200 {
			return Result{State: "uncertain", ErrorCode: "reconciliation-unavailable"}, nil
		}
		var rows []struct {
			Number      int64           `json:"number"`
			ID          int64           `json:"id"`
			Body        string          `json:"body"`
			URL         string          `json:"html_url"`
			PullRequest json.RawMessage `json:"pull_request"`
		}
		if json.Unmarshal(b, &rows) != nil {
			return Result{State: "uncertain", ErrorCode: "invalid-reconciliation"}, nil
		}
		for _, v := range rows {
			if len(v.PullRequest) > 0 || !strings.Contains(v.Body, j.Marker) {
				continue
			}
			id := v.Number
			if j.Kind == "github.comment" {
				id = v.ID
			}
			if id > 0 && strings.HasPrefix(v.URL, "https://github.com/"+j.Binding.Repository+"/issues/") {
				found = append(found, Result{State: "delivered", RemoteID: strconv.FormatInt(id, 10), RemoteURL: githubURL(j, id)})
			}
		}
		if len(found) > 1 {
			return Result{State: "uncertain", ErrorCode: "duplicate-markers"}, nil
		}
		if len(rows) < 100 {
			complete = true
			break
		}
	}
	if complete && len(found) == 1 {
		return found[0], nil
	}
	if !complete {
		return Result{State: "uncertain", ErrorCode: "reconciliation-budget-exceeded"}, nil
	}
	return Result{State: "uncertain", ErrorCode: "marker-not-visible"}, nil
}
