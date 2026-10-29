package model

import (
	"encoding/json"
	"testing"
)

func TestCanonicalHashPreservesValues(t *testing.T) {
	cmd := Command{Kind: "test", Meta: Mutation{ClientRequestID: "same"}, Payload: json.RawMessage(`{"a":1,"b":9007199254740993}`)}
	a, err := CanonicalHash(cmd)
	if err != nil || a == "" {
		t.Fatal("missing canonical hash")
	}
	cmd.Payload = json.RawMessage(`{ "b": 9007199254740993, "a": 1 }`)
	b, err := CanonicalHash(cmd)
	if err != nil || a != b {
		t.Fatal("JSON order changes semantic hash")
	}
	cmd.Payload = json.RawMessage(`{"a":1,"b":9007199254740992}`)
	c, _ := CanonicalHash(cmd)
	if a == c {
		t.Fatal("integer precision collision")
	}
}
