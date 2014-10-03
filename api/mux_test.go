package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/version"
)

func TestDefaultHandlers(t *testing.T) {
	tests := []struct {
		method string
		path   string
		code   int
	}{
		{"GET", "/", http.StatusMethodNotAllowed},
		{"GET", "/v1-alpha", http.StatusMethodNotAllowed},
		{"GET", "/bogus", http.StatusNotFound},
	}

	for i, tt := range tests {
		fr := registry.NewFakeRegistry()
		hdlr := NewServeMux(fr)
		rr := httptest.NewRecorder()

		req, err := http.NewRequest(tt.method, tt.path, nil)
		if err != nil {
			t.Errorf("case %d: failed setting up http.Request for test: %v", i, err)
			continue
		}

		hdlr.ServeHTTP(rr, req)

		err = assertErrorResponse(rr, tt.code)
		if err != nil {
			t.Errorf("case %d: %v", i, err)
		}

		wantServer := fmt.Sprintf("fleetd/%s", version.Version)
		gotServer := rr.HeaderMap["Server"][0]
		if wantServer != gotServer {
			t.Errorf("case %d: received incorrect Server header: want=%s, got=%s", i, wantServer, gotServer)
		}
	}
}
