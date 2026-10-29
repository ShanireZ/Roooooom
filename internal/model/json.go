package model

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

func RequireFields(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if e := json.Unmarshal(raw, &object); e != nil || object == nil {
		return Invalid("object required")
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Invalid("required field: " + field)
		}
	}
	return nil
}

func Decode(raw []byte, target any) error {
	if !utf8.Valid(raw) {
		return &DomainError{Code: "invalid-utf8", Status: 400, Message: "JSON input must use UTF-8 or ASCII Unicode escapes"}
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return Invalid("object required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(target); e != nil {
		return Invalid("invalid or unknown field")
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return Invalid("trailing JSON")
	}
	return nil
}
func JSON(value any) json.RawMessage {
	b, e := json.Marshal(value)
	if e != nil {
		panic(e)
	}
	return b
}
func Forbidden() error { return &DomainError{Code: "forbidden", Status: 403, Message: "access denied"} }
func NotFound() error {
	return &DomainError{Code: "not-found", Status: 404, Message: "record not found"}
}
