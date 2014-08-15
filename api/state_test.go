package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func TestUnitStateList(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fr.SetUnitStates([]unit.UnitState{
		unit.UnitState{UnitName: "XXX", ActiveState: "active"},
		unit.UnitState{UnitName: "YYY", ActiveState: "inactive"},
	})
	fAPI := &client.RegistryClient{fr}
	resource := &stateResource{fAPI, "/state"}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/state", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.list(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rw.Code)
	}

	ct := rw.HeaderMap["Content-Type"]
	if len(ct) != 1 {
		t.Errorf("Response has wrong number of Content-Type values: %v", ct)
	} else if ct[0] != "application/json" {
		t.Errorf("Expected application/json, got %s", ct)
	}

	if rw.Body == nil {
		t.Error("Received nil response body")
	} else {
		var page schema.UnitStatePage
		err := json.Unmarshal(rw.Body.Bytes(), &page)
		if err != nil {
			t.Fatalf("Received unparseable body: %v", err)
		}

		if len(page.States) != 2 {
			t.Errorf("Expected 2 UnitState objects, got %d", len(page.States))
			return
		}

		expect1 := &schema.UnitState{Name: "XXX", SystemdActiveState: "active"}
		if !reflect.DeepEqual(expect1, page.States[0]) {
			t.Errorf("expected first entity %#v, got %#v", expect1, page.States[0])
		}

		expect2 := &schema.UnitState{Name: "YYY", SystemdActiveState: "inactive"}
		if !reflect.DeepEqual(expect2, page.States[1]) {
			t.Errorf("expected first entity %#v, got %#v", expect2, page.States[1])
		}
	}
}
