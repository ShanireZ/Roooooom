package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"roooooom.local/roooooom/internal/coordinator"
	"roooooom.local/roooooom/internal/httpapi"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/store"
	"strings"
	"testing"
	"time"
)

func TestHTTPWaitAndSSE(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	_, e := f.store.Write(ctx, func(tx *store.Tx) (model.Receipt, error) {
		_, e := tx.Exec("INSERT INTO projects VALUES('p',1,'{}')")
		return model.Receipt{}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	owner, e := f.ids.Authenticate(ctx, f.cookie.Value)
	if e != nil {
		t.Fatal(e)
	}
	cap, e := f.ids.EnableEnrollment(ctx, owner, "p")
	if e != nil {
		t.Fatal(e)
	}
	credentials, e := f.ids.Enroll(ctx, cap, "worker", "worker")
	if e != nil {
		t.Fatal(e)
	}
	agent, e := f.ids.Authenticate(ctx, credentials.Token)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewUnstartedServer(nil)
	host := server.Listener.Addr().String()
	core := coordinator.New(f.store, nil, nil)
	server.Config.Handler = httpapi.New(core, f.ids, httpapi.Options{AllowedHost: host})
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 3 * time.Second}
	get := func(path string) *http.Response {
		t.Helper()
		r, _ := http.NewRequest("GET", server.URL+path, nil)
		r.Header.Set("Authorization", "Bearer "+credentials.Token)
		response, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		return response
	}
	initial, e := core.Sync(ctx, agent, "")
	if e != nil {
		t.Fatal(e)
	}
	message, e := core.Apply(ctx, owner, model.Command{Kind: "message.send", Meta: model.Mutation{ServiceID: f.store.Identity().ServiceID, Generation: f.store.Identity().Generation, ClientRequestID: "message"}, Payload: model.JSON(messagePayloadForWait(agent.ParticipantID))})
	if e != nil {
		t.Fatal(e)
	}
	response := get("/api/v1/wait?cursor=" + url.QueryEscape(initial.Cursor) + "&timeoutSeconds=1")
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("wait route", response.StatusCode)
	}
	var received struct{ Result coordinator.SyncPage }
	if e = json.NewDecoder(response.Body).Decode(&received); e != nil || len(received.Result.Events) != 1 {
		t.Fatal(received, e)
	}
	piggyback := get("/api/v1/queries/projects")
	var notice struct{ Unread []any }
	if e = json.NewDecoder(piggyback.Body).Decode(&notice); e != nil {
		t.Fatal(e)
	}
	piggyback.Body.Close()
	if len(notice.Unread) != 1 {
		t.Fatal("business response omitted unread reference")
	}
	stream := get("/api/v1/events?cursor=" + url.QueryEscape(initial.Cursor))
	defer stream.Body.Close()
	if stream.StatusCode != 200 || stream.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("SSE route", stream.StatusCode)
	}
	scanner := bufio.NewScanner(stream.Body)
	scanner.Buffer(make([]byte, 4096), 128*1024)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var replay coordinator.SyncPage
			if e = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &replay); e != nil {
				t.Fatal(e)
			}
			found = len(replay.Events) == 1
			break
		}
	}
	if !found {
		t.Fatal("SSE did not replay durable event")
	}
	stream.Body.Close()
	var state string
	if e = f.store.QueryRow(ctx, "SELECT state FROM message_receipts WHERE message_id=?", message.ResourceID).Scan(&state); e != nil || state != "stored" {
		t.Fatal("transport invented fetched", state, e)
	}
}
func messagePayloadForWait(recipient model.ID) map[string]any {
	return map[string]any{"projectId": "p", "threadId": "thread", "purpose": "discussion", "priority": "normal", "body": "HTTP witness", "recipientMode": "participants", "recipientIds": []model.ID{recipient}, "actionRequired": false}
}

func TestOwnerSSEStopsAfterCredentialRevocation(t *testing.T) {
	f := newAuthFixture(t)
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = httpapi.New(coordinator.New(f.store, nil, nil), f.ids, httpapi.Options{AllowedHost: server.Listener.Addr().String()})
	server.Start()
	defer server.Close()
	request, _ := http.NewRequest("GET", server.URL+"/api/v1/events", nil)
	request.AddCookie(f.cookie)
	client := &http.Client{Timeout: 3 * time.Second}
	response, e := client.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data: ") {
			break
		}
	}
	_, e = f.store.Write(t.Context(), func(tx *store.Tx) (model.Receipt, error) {
		if _, e := tx.Exec("UPDATE tokens SET revoked=1 WHERE kind='human'"); e != nil {
			return model.Receipt{}, e
		}
		seq, e := tx.AppendEvent("owner", model.JSON(map[string]any{"kind": "revocation-witness", "private": "must not reach revoked stream"}))
		return model.Receipt{EventID: seq}, e
	})
	if e != nil {
		t.Fatal(e)
	}
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "revocation-witness") {
			t.Fatal("revoked owner received a new private event")
		}
	}
	if e = scanner.Err(); e != nil {
		t.Fatal("revoked stream was not closed promptly", e)
	}
}
