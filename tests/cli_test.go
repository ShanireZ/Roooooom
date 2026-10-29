package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"roooooom.local/roooooom/internal/cli"
	"testing"
)

func TestCLIForbiddenCommands(t *testing.T) {
	for _, kind := range []string{"exec", "git", "terminal", "model", "host", "api-proxy"} {
		var out, err bytes.Buffer
		if code := cli.Run(context.Background(), []string{kind}, bytes.NewReader(nil), &out, &err); code != 2 {
			t.Fatal(kind, code)
		}
	}
	var out, err bytes.Buffer
	if code := cli.Run(context.Background(), []string{"help", "--json"}, bytes.NewReader(nil), &out, &err); code != 0 {
		t.Fatal("missing help contract", code, err.String())
	}
	var catalog map[string]any
	if json.Unmarshal(out.Bytes(), &catalog) != nil || catalog["protocolVersion"] != float64(1) {
		t.Fatal("invalid help JSON")
	}
}
