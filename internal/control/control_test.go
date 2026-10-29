package control

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func probe(t *testing.T, cfg Config) map[string]any {
	t.Helper()
	root := t.TempDir()
	cfg.Root, cfg.Address, cfg.Executable, cfg.ServiceID, cfg.Generation = root, "127.0.0.1:7339", filepath.Join(root, "core"), "s", "g"
	server, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	file, e := Read(root)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:7339/api/v1/control/probe", strings.NewReader(`{"serviceId":"s","datasetGeneration":"g"}`))
	r.Header.Set("Authorization", "Bearer "+file.Token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var body map[string]any
	if e = json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	return body
}

func TestProbeReportsOwnerLanguage(t *testing.T) {
	body := probe(t, Config{OwnerLanguage: func(context.Context) (string, error) { return "ja", nil }})
	if body["ownerLanguage"] != "ja" || body["serviceId"] != "s" || body["protocolVersion"] != float64(1) {
		t.Fatalf("probe %v", body)
	}
}

func TestProbeOmitsUnsetOwnerLanguage(t *testing.T) {
	body := probe(t, Config{})
	if _, ok := body["ownerLanguage"]; ok {
		t.Fatalf("probe without a language provider reported %v", body["ownerLanguage"])
	}
}
