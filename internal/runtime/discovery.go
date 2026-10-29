package runtime

import (
	"encoding/json"
	"roooooom.local/roooooom/internal/privatefs"
)

func (r *Runtime) Publish(address string) error {
	data, err := json.Marshal(map[string]any{"listenAddr": address, "serviceId": r.ServiceID, "datasetGeneration": r.Generation, "protocolVersion": 1})
	if err != nil {
		return err
	}
	return privatefs.WriteAtomic(r.Root, "discovery.json", data)
}
