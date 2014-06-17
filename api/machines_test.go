package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

func TestMachinesList(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fr.SetMachines([]machine.MachineState{
		{ID: "XXX", PublicIP: "", Metadata: nil},
		{ID: "YYY", PublicIP: "1.2.3.4", Metadata: map[string]string{"ping": "pong"}},
	})
	resource := &machinesResource{fr}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
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
		body := rw.Body.String()
		expected := `{"machines":[{"id":"XXX"},{"id":"YYY","metadata":{"ping":"pong"},"primaryIP":"1.2.3.4"}]}`
		if body != expected {
			t.Errorf("Expected body:\n%s\n\nReceived body:\n%s\n", expected, body)
		}
	}
}
