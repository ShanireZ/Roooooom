package cli_test

import (
	"bytes"
	"encoding/json"
	"roooooom.local/roooooom/internal/cli"
	"roooooom.local/roooooom/internal/client"
	"roooooom.local/roooooom/internal/model"
	"roooooom.local/roooooom/internal/testkit"
	"testing"
)

func TestNativeMutationUTF8InputBeforeOutbox(t *testing.T) {
	for _, kind := range []string{"invalid-gbk", "utf8", "ascii-escaped"} {
		t.Run(kind, func(t *testing.T) {
			data := t.TempDir()
			root, e := client.ProfileRoot(data, "encoding")
			if e != nil {
				t.Fatal(e)
			}
			profile := client.Profile{Root: root, Handle: "encoding", ProjectID: "synthetic", Mode: "offline", Provisional: true}
			if e = client.SaveProfile(profile); e != nil {
				t.Fatal(e)
			}
			payload := testkit.TaskPayload("synthetic", 0)
			payload.Goal = "進度"
			raw := model.JSON(map[string]any{"payload": payload})
			switch kind {
			case "invalid-gbk":
				raw = bytes.Replace(raw, []byte("進度"), []byte{0xdf, 0x4d, 0xb6, 0xc8}, 1)
			case "ascii-escaped":
				raw = bytes.Replace(raw, []byte("進度"), []byte(`\u9032\u5ea6`), 1)
			}
			var out, errOut bytes.Buffer
			exit := cli.Run(t.Context(), []string{"apply", "task.create", "--data-dir", data, "--profile", "encoding", "--json"}, bytes.NewReader(raw), &out, &errOut)
			if exit != 1 {
				t.Fatalf("unexpected exit=%d %s", exit, out.String())
			}
			var response struct {
				Error struct {
					Code   string
					Status int
				}
				DraftID string `json:"draftId"`
			}
			if e = json.Unmarshal(out.Bytes(), &response); e != nil {
				t.Fatal(e)
			}
			c := client.New(profile, nil)
			defer c.Close()
			pending, e := c.Pending(t.Context())
			if e != nil {
				t.Fatal(e)
			}
			if kind == "invalid-gbk" {
				if response.Error.Code != "invalid-utf8" || response.Error.Status != 400 || response.DraftID != "" || len(pending) != 0 {
					t.Fatalf("malformed input became durable draft: %s", out.String())
				}
			} else {
				if response.Error.Code != "offline" || response.DraftID == "" || len(pending) != 1 {
					t.Fatalf("valid text did not retain an offline draft: %s", out.String())
				}
				var saved model.TaskCreate
				if e = json.Unmarshal(pending[0].Command.Payload, &saved); e != nil || saved.Goal != "進度" {
					t.Fatal("valid Chinese text changed", e)
				}
			}
		})
	}
}
