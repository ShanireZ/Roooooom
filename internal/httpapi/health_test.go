package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHealthMinimalIdentity(t *testing.T) {
	h := Health("service", "generation")
	for _, path := range []string{"/health", "/identity"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		var body map[string]any
		if w.Code != 200 {
			t.Fatalf("status %d", w.Code)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["serviceId"] != "service" || body["datasetGeneration"] != "generation" {
			t.Fatal(body)
		}
		if len(body) > 4 {
			t.Fatal("diagnostics expose extra state", body)
		}
	}
}
