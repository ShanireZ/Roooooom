package api

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestDailyContractSubjectAndOwnerBoundary(t *testing.T) {
	var doc struct {
		Paths      map[string]json.RawMessage
		Components struct {
			Schemas map[string]struct {
				Required   []string
				Properties map[string]json.RawMessage
				OneOf      []json.RawMessage
			}
		}
	}
	if e := json.Unmarshal(Contract, &doc); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"DecisionRequest", "DecisionRequestView"} {
		schema := doc.Components.Schemas[name]
		if slices.Contains(schema.Required, "taskId") || schema.Properties["requestId"] == nil || len(schema.OneOf) != 2 {
			t.Fatal(name, "must publish exactly one intake/task subject")
		}
	}
	if doc.Components.Schemas["WorkRequest"].Properties["preparationEpoch"] == nil || doc.Components.Schemas["Task"].Properties["sourceRequestText"] == nil {
		t.Fatal("source constraint contract absent")
	}
	var route struct {
		Get struct{ Security []map[string]any }
	}
	if e := json.Unmarshal(doc.Paths["/api/v1/setup/connection"], &route); e != nil {
		t.Fatal("missing guided route")
	}
	if len(route.Get.Security) != 1 || route.Get.Security[0]["OwnerCookie"] == nil {
		t.Fatal("setup metadata must be owner-only")
	}
}
