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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

func fakeMachinesSetup() (*machinesResource, *httptest.ResponseRecorder) {
	fr := registry.NewFakeRegistry()
	fr.SetMachines([]machine.MachineState{
		{ID: "XXX", PublicIP: "", Metadata: map[string]string{}},
		{ID: "YYY", PublicIP: "1.2.3.4", Metadata: map[string]string{"ping": "pong"}},
	})
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &machinesResource{cAPI: fAPI}
	rw := httptest.NewRecorder()

	return resource, rw
}

func TestMachinesList(t *testing.T) {
	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("GET", "http://example.com/machines", nil)
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

func TestMachinesListBadMethod(t *testing.T) {
	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("POST", "http://example.com/machines", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)

	err = assertErrorResponse(rw, http.StatusMethodNotAllowed)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestMachinesListBadNextPageToken(t *testing.T) {
	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("GET", "http://example.com/machines?nextPageToken=EwBMLg==", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)

	err = assertErrorResponse(rw, http.StatusBadRequest)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestExtractMachinePage(t *testing.T) {
	all := make([]machine.MachineState, 103)
	for i := 0; i < 103; i++ {
		id := strconv.FormatInt(int64(i), 10)
		all[i] = machine.MachineState{ID: id}
	}

	tests := []struct {
		token    PageToken
		idxStart int
		idxEnd   int
		next     *PageToken
	}{
		{PageToken{Page: 1, Limit: 60}, 0, 59, &PageToken{Page: 2, Limit: 60}},
		{PageToken{Page: 2, Limit: 60}, 60, 102, nil},
	}

	for i, tt := range tests {
		page := extractMachinePage(all, tt.token)
		expectCount := (tt.idxEnd - tt.idxStart + 1)
		if len(page.Machines) != expectCount {
			t.Fatalf("case %d: expected page of %d, got %d", i, expectCount, len(page.Machines))
		}

		first := page.Machines[0].Id
		if first != strconv.FormatInt(int64(tt.idxStart), 10) {
			t.Errorf("case %d: first element in page should have ID %d, got %s", i, tt.idxStart, first)
		}

		last := page.Machines[len(page.Machines)-1].Id
		if last != strconv.FormatInt(int64(tt.idxEnd), 10) {
			t.Errorf("case %d: first element in page should have ID %d, got %s", i, tt.idxEnd, last)
		}

		if tt.next == nil && page.NextPageToken != "" {
			t.Errorf("case %d: did not expect NextPageToken", i)
			continue
		} else if page.NextPageToken == "" {
			if tt.next != nil {
				t.Errorf("case %d: did not receive expected NextPageToken", i)
			}
			continue
		}

		next, err := decodePageToken(page.NextPageToken)
		if err != nil {
			t.Errorf("case %d: unable to parse NextPageToken: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(next, tt.next) {
			t.Errorf("case %d: expected PageToken %v, got %v", i, tt.next, next)
		}
	}
}

func TestMachinesPatchAddModify(t *testing.T) {
	reqBody := `
	[{"op": "add", "path": "/XXX/metadata/foo", "value": "bar"},
	 {"op": "replace", "path": "/YYY/metadata/ping", "value": "splat"}]
	`

	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("PATCH", "http://example.com/machines", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
	if rw.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rw.Code)
	}

	// fetch machine to make sure data has been added
	req, err = http.NewRequest("GET", "http://example.com/machines", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}
	rw.Body.Reset()
	resource.ServeHTTP(rw, req)

	if rw.Body == nil {
		t.Error("Received nil response body")
	} else {
		body := rw.Body.String()
		expected := `{"machines":[{"id":"XXX","metadata":{"foo":"bar"}},{"id":"YYY","metadata":{"ping":"splat"},"primaryIP":"1.2.3.4"}]}`
		if body != expected {
			t.Errorf("Expected body:\n%s\n\nReceived body:\n%s\n", expected, body)
		}
	}
}

func TestMachinesPatchDelete(t *testing.T) {
	reqBody := `
	[{"op": "remove", "path": "/XXX/metadata/foo"},
	 {"op": "remove", "path": "/YYY/metadata/ping"}]
	`

	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("PATCH", "http://example.com/machines", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
	if rw.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rw.Code)
	}

	// fetch machine to make sure data has been added
	req, err = http.NewRequest("GET", "http://example.com/machines", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}
	rw.Body.Reset()
	resource.ServeHTTP(rw, req)

	if rw.Body == nil {
		t.Error("Received nil response body")
	} else {
		body := rw.Body.String()
		expected := `{"machines":[{"id":"XXX"},{"id":"YYY","primaryIP":"1.2.3.4"}]}`
		if body != expected {
			t.Errorf("Expected body:\n%s\n\nReceived body:\n%s\n", expected, body)
		}
	}
}

func TestMachinesPatchBadOp(t *testing.T) {
	reqBody := `
	[{"op": "noop", "path": "/XXX/metadata/foo", "value": "bar"}]
	`

	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("PATCH", "http://example.com/machines", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rw.Code)
	}
}

func TestMachinesPatchBadPath(t *testing.T) {
	reqBody := `
	[{"op": "add", "path": "/XXX/foo", "value": "bar"}]
	`

	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("PATCH", "http://example.com/machines", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rw.Code)
	}
}

func TestMachinesPatchBadValue(t *testing.T) {
	reqBody := `
	[{"op": "add", "path": "/XXX/foo"}]
	`

	resource, rw := fakeMachinesSetup()
	req, err := http.NewRequest("PATCH", "http://example.com/machines", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rw.Code)
	}
}
