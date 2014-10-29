/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coreos/fleet/schema"
)

func TestDiscoveryJson(t *testing.T) {
	url := "http://example.com/discovery"
	for _, verb := range []string{"POST", "PUT", "DELETE"} {
		res := &discoveryResource{}
		rw := httptest.NewRecorder()
		req, err := http.NewRequest(verb, url, nil)
		if err != nil {
			t.Fatalf("Failed creating http.Request: %v", err)
		}
		res.ServeHTTP(rw, req)
		if rw.Code != http.StatusBadRequest {
			t.Errorf("Expected 400 for %s, got %d", verb, rw.Code)
		}
	}
	res := &discoveryResource{}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}
	res.ServeHTTP(rw, req)
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
		if rw.Body.String() != schema.DiscoveryJSON {
			t.Error("Received unexpected body!")
		}
	}
}
