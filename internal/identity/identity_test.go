package identity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"strings"
	"testing"
	"time"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }
func fixture(t *testing.T) (*Service, *store.Store, *testClock) {
	t.Helper()
	st, e := store.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	clock := &testClock{time.Now().UTC()}
	s := New(st, clock)
	s.SetOrigin("http://127.0.0.1:7337")
	return s, st, clock
}
func activate(t *testing.T, s *Service) (model.Actor, string, string) {
	t.Helper()
	path, e := s.IssueOwnerBootstrap(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	match := regexp.MustCompile(`name="capability" value="([A-Za-z0-9_-]+)"`).FindSubmatch(b)
	if len(match) != 2 {
		t.Fatal("missing capability form")
	}
	token, csrf, e := s.Activate(context.Background(), string(match[1]))
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Authenticate(context.Background(), token)
	if e != nil {
		t.Fatal(e)
	}
	return a, token, csrf
}
func TestOneTimeOwnerActivation(t *testing.T) {
	s, _, clock := fixture(t)
	ctx := context.Background()
	path, e := s.IssueOwnerBootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	match := regexp.MustCompile(`name="capability" value="([A-Za-z0-9_-]+)"`).FindSubmatch(b)
	if len(match) != 2 {
		t.Fatal("no capability")
	}
	token, csrf, e := s.Activate(ctx, string(match[1]))
	if e != nil || token == "" || csrf == "" {
		t.Fatal(e)
	}
	if _, _, e = s.Activate(ctx, string(match[1])); model.Status(e) != 409 {
		t.Fatal("replayed activation", e)
	}
	path, e = s.IssueOwnerBootstrap(ctx)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(path)
	match = regexp.MustCompile(`name="capability" value="([A-Za-z0-9_-]+)"`).FindSubmatch(b)
	clock.now = clock.now.Add(6 * time.Minute)
	if _, _, e = s.Activate(ctx, string(match[1])); model.Status(e) != 409 {
		t.Fatal("expired activation", e)
	}
}

func TestLauncherOwnerActivationAtExpiry(t *testing.T) {
	s, _, clock := fixture(t)
	clock.now = clock.now.Truncate(time.Second)
	if needed, e := s.NeedsOwnerActivation(t.Context()); e != nil || !needed {
		t.Fatal(needed, e)
	}
	activate(t, s)
	if needed, e := s.NeedsOwnerActivation(t.Context()); e != nil || needed {
		t.Fatal(needed, e)
	}
	clock.now = clock.now.Add(24*time.Hour + time.Nanosecond)
	if needed, e := s.NeedsOwnerActivation(t.Context()); e != nil || !needed {
		t.Fatal("expired owner still considered active", needed, e)
	}
}
func TestScopeRevokedAndEnrollmentCannotBypass(t *testing.T) {
	s, st, _ := fixture(t)
	ctx := context.Background()
	owner, _, _ := activate(t, s)
	_, e := st.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	cap, e := s.EnableEnrollment(ctx, owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := s.Enroll(ctx, cap, "request-1", "worker")
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Enroll(ctx, cap, "request-1", "worker")
	if e != nil || again.Token != credentials.Token {
		t.Fatal("enrollment replay", e)
	}
	a, e := s.Authenticate(ctx, credentials.Token)
	if e != nil || a.Kind != "agent" || len(a.ProjectIDs) != 1 || a.ProjectIDs[0] != "p" {
		t.Fatal(a, e)
	}
	if _, e = s.EnableEnrollment(ctx, a, "p"); model.Status(e) != 403 {
		t.Fatal("agent elevated", e)
	}
	if e = s.Revoke(ctx, owner, a.ParticipantID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Authenticate(ctx, credentials.Token); model.Status(e) != 401 {
		t.Fatal("revoked token accepted", e)
	}
	if _, e = s.Enroll(ctx, cap, "request-2", "renamed"); model.Status(e) != 401 {
		t.Fatal("revocation bypass", e)
	}
}
func TestPrivateCredentialPermissions(t *testing.T) {
	s, st, _ := fixture(t)
	owner, secret, _ := activate(t, s)
	if owner.Kind != "human" {
		t.Fatal(owner)
	}
	var body string
	if e := st.QueryRow(context.Background(), "SELECT group_concat(body) FROM tokens").Scan(&body); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(body, secret) {
		t.Fatal("plaintext token in database")
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(st.Path), "credentials", "owner.json"))
	if e != nil {
		t.Fatal(e)
	}
	var saved map[string]string
	if json.Unmarshal(b, &saved) != nil || saved["token"] != secret {
		t.Fatal("private credential missing")
	}
	// Platform-specific ACL/mode behavior is exercised by privatefs permission tests.
}
