package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

func TestMachineHelpIncludesExplicitRecovery(t *testing.T) {
	var out bytes.Buffer
	if Run(t.Context(), []string{"help", "--json"}, bytes.NewReader(nil), &out, &out) != 0 {
		t.Fatal("help failed")
	}
	var help struct{ CLI []string }
	if json.Unmarshal(out.Bytes(), &help) != nil || !slices.Contains(help.CLI, "recover") {
		t.Fatal("recovery is missing from machine-readable CLI help")
	}
}

func TestMachineHelpPublishesIntakeWorkflow(t *testing.T) {
	var out bytes.Buffer
	if Run(t.Context(), []string{"help", "--json"}, bytes.NewReader(nil), &out, &out) != 0 {
		t.Fatal("help failed")
	}
	var help struct {
		CLI      []string
		Commands map[string]json.RawMessage
		Queries  []string
	}
	if e := json.Unmarshal(out.Bytes(), &help); e != nil {
		t.Fatal(e)
	}
	if !slices.Contains(help.CLI, "request") || !slices.Contains(help.Queries, "requests") || !slices.Contains(help.Queries, "request") {
		t.Fatal("intake discovery absent")
	}
	for _, kind := range []string{"request.create", "request.claim", "request.release", "request.handoff", "request.accept", "request.resume", "request.assign", "request.cancel", "request.convert", "binding.inspect"} {
		if help.Commands[kind] == nil {
			t.Fatal(kind)
		}
	}
}
