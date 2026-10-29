package coordinator_test

import (
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"strings"
	"testing"
)

func messagePayload(project, task model.ID, mode string, recipients []model.ID, body string) map[string]any {
	return map[string]any{"projectId": project, "taskId": task, "threadId": "synthetic-thread", "purpose": "question", "priority": "normal", "body": body, "recipientMode": mode, "recipientIds": recipients, "actionRequired": true}
}
func TestReceiptAndRoleRecipientSnapshot(t *testing.T) {
	f, a, b, task, assignment, _ := responsibilityFixture(t, 0)
	project := f.Task(task).ProjectID
	message := f.Must(b, "message.send", "", messagePayload(project, task, "integrator", []model.ID{}, "Please review the interface"), f.Meta(b))
	offer := propose(t, f, a, b, task, handoffPayload(a, b, assignment, "release"))
	accept(t, f, b, task, offer)
	history, e := f.Core.Read(context.Background(), b, model.Query{Kind: "thread", TargetID: task})
	if e != nil {
		t.Fatal("new integrator cannot read task history", e)
	}
	var messages []map[string]any
	if e = json.Unmarshal(history.Data, &messages); e != nil || len(messages) != 1 {
		t.Fatal(messages, e)
	}
	var recipient model.ID
	var state string
	if e := f.Store.QueryRow(context.Background(), "SELECT participant_id,state FROM message_receipts WHERE message_id=?", message.ResourceID).Scan(&recipient, &state); e != nil {
		t.Fatal(e)
	}
	if recipient != a.ParticipantID || state != "stored" {
		t.Fatal("recipient moved or fictitious read", recipient, state)
	}
	m := f.Meta(b)
	m.ExpectedVersion = 1
	if _, e := f.Apply(b, "receipt.update", message.ResourceID, map[string]any{"state": "seen"}, m); model.Status(e) != 403 {
		t.Fatal("new integrator forged old recipient read", e)
	}
	m = f.Meta(a)
	m.ExpectedVersion = 1
	f.Must(a, "receipt.update", message.ResourceID, map[string]any{"state": "fetched"}, m)
	if e := f.Store.QueryRow(context.Background(), "SELECT state FROM message_receipts WHERE message_id=?", message.ResourceID).Scan(&state); e != nil || state != "fetched" {
		t.Fatal("fetched is not seen", state, e)
	}
	m = f.Meta(a)
	m.ExpectedVersion = 2
	f.Must(a, "receipt.update", message.ResourceID, map[string]any{"state": "seen"}, m)
}
func TestInboxPaginationAndMessageReferences(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "sender"), f.Agent(p, "reader")
	ids := map[model.ID]bool{}
	for range 3 {
		r := f.Must(a, "message.send", "", messagePayload(p, "", "participants", []model.ID{b.ParticipantID}, "one message"), f.Meta(a))
		ids[r.ResourceID] = true
	}
	cursor := ""
	seen := 0
	for {
		page, e := f.Core.Read(context.Background(), b, model.Query{Kind: "inbox", Cursor: cursor, Limit: 1})
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct{ ID model.ID }
		if e = json.Unmarshal(page.Data, &rows); e != nil {
			t.Fatal(e)
		}
		for _, row := range rows {
			if !ids[row.ID] {
				t.Fatal("unknown or repeated message")
			}
			delete(ids, row.ID)
			seen++
		}
		if !page.HasMore {
			break
		}
		if page.Cursor == "" {
			t.Fatal("missing inbox continuation")
		}
		cursor = page.Cursor
	}
	if seen != 3 {
		t.Fatal("inbox page loss", seen)
	}
}
func TestMetadataQueryCursorDoesNotRepeat(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	for range 3 {
		f.Must(f.Owner(), "task.create", "", testkit.TaskPayload(p, 0), f.Meta(f.Owner()))
	}
	seen := map[model.ID]bool{}
	cursor := ""
	for range 4 {
		page, e := f.Core.Read(context.Background(), f.Owner(), model.Query{Kind: "tasks", ProjectID: p, Limit: 1, Cursor: cursor})
		if e != nil {
			t.Fatal(e)
		}
		var tasks []model.Task
		if e = json.Unmarshal(page.Data, &tasks); e != nil {
			t.Fatal(e)
		}
		for _, task := range tasks {
			if seen[task.ID] {
				t.Fatal("metadata query repeated cursor")
			}
			seen[task.ID] = true
		}
		if !page.HasMore {
			break
		}
		cursor = page.Cursor
	}
	if len(seen) != 3 {
		t.Fatal("metadata cursor skipped tasks")
	}
}
func TestResponseLossIsNotFetched(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "sender"), f.Agent(p, "recipient")
	message := f.Must(a, "message.send", "", messagePayload(p, "", "participants", []model.ID{b.ParticipantID}, "durable until received"), f.Meta(a))
	first, e := f.Core.Sync(context.Background(), b, "")
	if e != nil {
		t.Fatal(e)
	}
	again, e := f.Core.Sync(context.Background(), b, "")
	if e != nil {
		t.Fatal(e)
	}
	if len(first.Events) == 0 || len(first.Events) != len(again.Events) {
		t.Fatal("response-loss replay")
	}
	var state string
	if e = f.Store.QueryRow(context.Background(), "SELECT state FROM message_receipts WHERE message_id=?", message.ResourceID).Scan(&state); e != nil || state != "stored" {
		t.Fatal("prepared response marked fetched", state, e)
	}
}
func TestReceiptTransitionsAndBroadcastScope(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "sender"), f.Agent(p, "recipient")
	payload := messagePayload(p, "", "project", []model.ID{}, "announcement")
	if _, e := f.Apply(a, "message.send", "", payload, f.Meta(a)); model.Status(e) != 403 {
		t.Fatal("worker broadcast", e)
	}
	message := f.Must(a, "message.send", "", messagePayload(p, "", "participants", []model.ID{b.ParticipantID}, "Please handle this"), f.Meta(a))
	m := f.Meta(b)
	m.ExpectedVersion = 1
	if _, e := f.Apply(b, "receipt.update", message.ResourceID, map[string]any{"state": "seen"}, m); model.Status(e) != 409 {
		t.Fatal("seen before fetched", e)
	}
	for i, state := range []string{"fetched", "seen", "accepted"} {
		m = f.Meta(b)
		m.ExpectedVersion = int64(i + 1)
		f.Must(b, "receipt.update", message.ResourceID, map[string]any{"state": state, "note": "explicit next step"}, m)
	}
	m = f.Meta(b)
	m.ExpectedVersion = 4
	if _, e := f.Apply(b, "receipt.update", message.ResourceID, map[string]any{"state": "resolved", "note": "done"}, m); model.Status(e) != 409 {
		t.Fatal("resolved without result evidence", e)
	}
	f.Must(b, "receipt.update", message.ResourceID, map[string]any{"state": "resolved", "note": "answer recorded", "evidenceRefs": []string{"synthetic://answer"}}, m)
	m = f.Meta(b)
	m.ExpectedVersion = 5
	if _, e := f.Apply(b, "receipt.update", message.ResourceID, map[string]any{"state": "fetched"}, m); model.Status(e) != 409 {
		t.Fatal("receipt regressed", e)
	}
	view, e := f.Core.Read(t.Context(), b, model.Query{Kind: "inbox"})
	if e != nil {
		t.Fatal(e)
	}
	var inbox []any
	if e = json.Unmarshal(view.Data, &inbox); e != nil || len(inbox) != 0 {
		t.Fatal("resolved action remains pending", e)
	}
}
func TestCursorPagingAndReset(t *testing.T) {
	f := testkit.New(t)
	p, _ := f.Project()
	a, b := f.Agent(p, "sender"), f.Agent(p, "recipient")
	ctx := context.Background()
	initial, e := f.Core.Sync(ctx, b, "")
	if e != nil {
		t.Fatal(e)
	}
	cursor := initial.Cursor
	for range 105 {
		f.Must(a, "message.send", "", messagePayload(p, "", "participants", []model.ID{b.ParticipantID}, strings.Repeat("x", 1024)), f.Meta(a))
	}
	count := 0
	for {
		page, e := f.Core.Sync(ctx, b, cursor)
		if e != nil {
			t.Fatal(e)
		}
		encoded := model.JSON(page)
		if len(page.Events) > 100 || len(encoded) > 64*1024 || page.Bytes > 64*1024 {
			t.Fatal("unbounded page", len(encoded))
		}
		for _, event := range page.Events {
			var body struct{ Kind string }
			if e = json.Unmarshal(event.Body, &body); e != nil {
				t.Fatal(e)
			}
			if body.Kind == "message.send" {
				count++
			}
		}
		cursor = page.Cursor
		if !page.HasMore {
			break
		}
	}
	if count != 105 {
		t.Fatal("lost events", count)
	}
	if _, e = f.Core.Sync(ctx, a, cursor); model.Status(e) != 409 {
		t.Fatal("cross-actor cursor", e)
	}
	changed := cursor[:len(cursor)-1] + "!"
	if _, e = f.Core.Sync(ctx, b, changed); model.Status(e) != 409 {
		t.Fatal("forged cursor", e)
	}
	big := messagePayload(p, "", "participants", []model.ID{b.ParticipantID}, strings.Repeat("\x00", 16*1024))
	f.Must(a, "message.send", "", big, f.Meta(a))
	page, e := f.Core.Sync(ctx, b, cursor)
	if e != nil || len(page.Events) != 1 || len(model.JSON(page)) > 64*1024 {
		t.Fatal("complete-message reference", e)
	}
	big["body"] = strings.Repeat("x", 16*1024+1)
	if _, e = f.Apply(a, "message.send", "", big, f.Meta(a)); model.Status(e) != 400 {
		t.Fatal("oversize message accepted", e)
	}
}
