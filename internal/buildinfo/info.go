package buildinfo

import (
	"encoding/base64"
	"encoding/json"
	"runtime"
	"runtime/debug"
)

// Encoded is supplied by the reproducible maintainer build, never runtime input.
var Encoded string

func Current() map[string]any {
	metadata := map[string]any{"built": false}
	if raw, e := base64.RawURLEncoding.DecodeString(Encoded); e == nil && len(raw) > 0 {
		if json.Unmarshal(raw, &metadata) != nil {
			metadata = map[string]any{"built": false}
		}
	}
	cgo := "unknown"
	if b, ok := debug.ReadBuildInfo(); ok {
		for _, s := range b.Settings {
			if s.Key == "CGO_ENABLED" {
				cgo = s.Value
			}
		}
	}
	return map[string]any{"metadata": metadata, "goVersion": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH, "cgoEnabled": cgo != "0"}
}
