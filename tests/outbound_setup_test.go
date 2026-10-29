package tests

import (
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"testing"
)

func TestGuidedInspectionHTTPAuthority(t *testing.T) {
	f := newAuthFixture(t)
	_, e := f.store.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	owner, e := f.ids.Authenticate(t.Context(), f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(t.Context(), owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := f.ids.Enroll(t.Context(), cap, "inspect-agent", "inspect-agent")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"binding.inspect", "binding.credential"} {
		r := f.request("POST", "/api/v1/commands/"+kind, "not json", agent.Token, "")
		if r.Code != 403 {
			t.Fatalf("%s decoded agent input: %d", kind, r.Code)
		}
	}
	r := f.request("POST", "/api/v1/commands/binding.inspect", "{}", "owner", "")
	if r.Code != 403 {
		t.Fatal("inspection bypassed CSRF", r.Code)
	}
}
