package httpapi

import (
	"encoding/json"
	"net/http"
)

func Health(serviceID, generation string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" && r.URL.Path != "/identity" {
			http.NotFound(w, r)
			return
		}
		if r.Method != "GET" {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "serviceId": serviceID, "datasetGeneration": generation, "protocolVersion": 1})
	})
}
