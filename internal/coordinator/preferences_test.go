package coordinator_test

import (
	"context"
	"testing"

	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
)

func setLanguage(f *testkit.Fixture, a model.Actor, language string, meta model.Mutation) (model.Receipt, error) {
	return f.Core.Preferences(context.Background(), a, model.Command{Kind: "owner.language", Payload: model.JSON(map[string]string{"language": language}), Meta: meta})
}

func TestOwnerLanguageSetAndClear(t *testing.T) {
	f := testkit.New(t)
	ctx := context.Background()
	if got, e := f.Store.OwnerLanguage(ctx); e != nil || got != "" {
		t.Fatalf("initial owner language %q, %v", got, e)
	}
	if _, e := setLanguage(f, f.Owner(), "ja", f.Meta(f.Owner())); e != nil {
		t.Fatal(e)
	}
	if got, e := f.Store.OwnerLanguage(ctx); e != nil || got != "ja" {
		t.Fatalf("owner language %q, %v", got, e)
	}
	if _, e := setLanguage(f, f.Owner(), "", f.Meta(f.Owner())); e != nil {
		t.Fatal(e)
	}
	if got, e := f.Store.OwnerLanguage(ctx); e != nil || got != "" {
		t.Fatalf("cleared owner language %q, %v", got, e)
	}
}

func TestOwnerLanguageAgentForbidden(t *testing.T) {
	f := testkit.New(t)
	project, _ := f.Project()
	agent := f.Agent(project, "worker")
	if _, e := setLanguage(f, agent, "en", f.Meta(agent)); model.Status(e) != 403 {
		t.Fatalf("agent set owner language: %v", e)
	}
}

func TestOwnerLanguageInvalid(t *testing.T) {
	f := testkit.New(t)
	for _, value := range []string{"fr", "zh", "EN", "zh-Hant"} {
		if _, e := setLanguage(f, f.Owner(), value, f.Meta(f.Owner())); model.Status(e) != 400 {
			t.Fatalf("%q accepted: %v", value, e)
		}
	}
}

func TestOwnerLanguageRetryReturnsReceipt(t *testing.T) {
	f := testkit.New(t)
	meta := f.Meta(f.Owner())
	first, e := setLanguage(f, f.Owner(), "zh-TW", meta)
	if e != nil {
		t.Fatal(e)
	}
	again, e := setLanguage(f, f.Owner(), "zh-TW", meta)
	if e != nil || again.EventID != first.EventID || again.RequestID != first.RequestID {
		t.Fatalf("retry receipt %+v vs %+v, %v", again, first, e)
	}
	if _, e = setLanguage(f, f.Owner(), "ja", meta); model.Status(e) != 409 {
		t.Fatalf("same request identity with a different payload: %v", e)
	}
}

func TestOwnerLanguageEmitsEvent(t *testing.T) {
	f := testkit.New(t)
	if _, e := setLanguage(f, f.Owner(), "en", f.Meta(f.Owner())); e != nil {
		t.Fatal(e)
	}
	var n int
	if e := f.Store.QueryRow(context.Background(), "SELECT count(*) FROM events WHERE json_extract(body,'$.kind')='owner.language'").Scan(&n); e != nil || n != 1 {
		t.Fatalf("owner.language events %d, %v", n, e)
	}
}
