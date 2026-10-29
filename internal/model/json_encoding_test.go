package model

import (
	"reflect"
	"testing"
)

func TestDecodeRejectsNonUTF8BeforeReplacingText(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte("{\"text\":\"\xdfM\"}"), // GBK bytes for 進; reproduces the observed Windows producer failure.
		[]byte("{\"text\":\"\xff\"}"),
		[]byte("{\"\xff\":\"value\"}"),
	} {
		target := map[string]string{"original": "preserve"}
		e := Decode(raw, &target)
		if e == nil || Status(e) != 400 {
			t.Fatalf("non-UTF8 JSON accepted and may be silently replaced: %v", e)
		}
		if !reflect.DeepEqual(target, map[string]string{"original": "preserve"}) {
			t.Fatal("invalid input mutated target before rejection")
		}
	}
}

func TestDecodePreservesUTF8AndASCIIUnicodeEscapes(t *testing.T) {
	for _, raw := range []string{`{"text":"進度 🚀"}`, `{"text":"\u9032\u5ea6 \ud83d\ude80"}`} {
		var value struct {
			Text string `json:"text"`
		}
		if e := Decode([]byte(raw), &value); e != nil {
			t.Fatal(e)
		}
		if value.Text != "進度 🚀" {
			t.Fatalf("text changed: %q", value.Text)
		}
	}
}
