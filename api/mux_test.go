package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/fleet/registry"
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

		if !strings.HasPrefix(rr.HeaderMap["Server"][0], "fleetd") {
			t.Errorf("wrong Server header found %v", rr.HeaderMap["Server"])
		}
	}
}
