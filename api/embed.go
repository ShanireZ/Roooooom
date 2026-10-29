package api

import _ "embed"

// Contract is the authoritative JSON-compatible YAML document embedded in the native CLI.
//
//go:embed openapi.yaml
var Contract []byte
