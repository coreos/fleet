// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		{"GET", "/fleet/v1", http.StatusMethodNotAllowed},
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
