package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/privatefs"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestOnlyDecodedClientMessagesBecomeFetched(t *testing.T) {
	f := testkit.New(t)
	project, _ := f.Project()
	sender := f.Agent(project, "sender")
	cap, e := f.Identities.EnableEnrollment(t.Context(), f.Owner(), project)
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.Identities.Enroll(t.Context(), cap, "reader", "reader")
	if e != nil {
		t.Fatal(e)
	}
	message := f.Must(sender, "message.send", "", map[string]any{"projectId": project, "threadId": "thread", "purpose": "discussion", "priority": "normal", "body": "client decoding witness", "recipientMode": "participants", "recipientIds": []model.ID{credentials.ParticipantID}, "actionRequired": false}, f.Meta(sender))
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = httpapi.New(f.Core, f.Identities, httpapi.Options{AllowedHost: server.Listener.Addr().String()})
	server.Start()
	defer server.Close()
	root := filepath.Join(t.TempDir(), "client")
	if e = privatefs.WriteAtomic(root, "credentials/session.json", model.JSON(credentials)); e != nil {
		t.Fatal(e)
	}
	identity := f.Store.Identity()
	p := Profile{Root: root, Handle: "reader", ServiceURL: server.URL, ServiceID: identity.ServiceID, Generation: identity.Generation, ProjectID: project, ParticipantID: credentials.ParticipantID, SessionID: credentials.SessionID, CredentialRef: "credentials/session.json", Mode: "online"}
	dropped := false
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response, e := http.DefaultTransport.RoundTrip(r)
		if e == nil && !dropped && r.URL.Path == "/api/v1/sync" {
			dropped = true
			response.Body.Close()
			return nil, io.ErrUnexpectedEOF
		}
		return response, e
	})
	c := New(p, transport)
	defer c.Close()
	if _, e = c.Sync(t.Context()); e == nil {
		t.Fatal("response loss was not observed")
	}
	state := func() string {
		var value string
		if e := f.Store.QueryRow(t.Context(), "SELECT state FROM message_receipts WHERE message_id=? AND participant_id=?", message.ResourceID, credentials.ParticipantID).Scan(&value); e != nil {
			t.Fatal(e)
		}
		return value
	}
	if state() != "stored" {
		t.Fatal("undecoded response marked fetched")
	}
	if _, e = c.Sync(t.Context()); e != nil {
		t.Fatal(e)
	}
	if state() != "fetched" {
		t.Fatal("decoded message was not fetched, or was automatically seen")
	}
}
