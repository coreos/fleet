package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coreos/fleet/registry"
)

func TestHealthCheckPass(t *testing.T) {
	fr := registry.NewFakeRegistry()
	resource := &healthResource{fr}
	rw := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "http://example.com/health", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rw.Code)
	}

	if rw.Body == nil {
		t.Error("Received nil response body")
	} else {
		expect := "OK"
		output := rw.Body.String()
		if output != expect {
			t.Errorf("Expected %q, got %q", expect, output)
		}
	}
}
