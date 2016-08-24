// Copyright 2014 The fleet Authors
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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func TestUnitStateList(t *testing.T) {
	us1 := unit.UnitState{UnitName: "AAA", ActiveState: "active"}
	us2 := unit.UnitState{UnitName: "BBB", ActiveState: "inactive", MachineID: "XXX"}
	us3 := unit.UnitState{UnitName: "CCC", ActiveState: "active", MachineID: "XXX"}
	us4 := unit.UnitState{UnitName: "CCC", ActiveState: "inactive", MachineID: "YYY"}
	sus1 := &schema.UnitState{Name: "AAA", SystemdActiveState: "active"}
	sus2 := &schema.UnitState{Name: "BBB", SystemdActiveState: "inactive", MachineID: "XXX"}
	sus3 := &schema.UnitState{Name: "CCC", SystemdActiveState: "active", MachineID: "XXX"}
	sus4 := &schema.UnitState{Name: "CCC", SystemdActiveState: "inactive", MachineID: "YYY"}

	for i, tt := range []struct {
		url      string
		expected []*schema.UnitState
	}{
		{
			// Standard query - return all results
			"http://example.com/state",
			[]*schema.UnitState{sus1, sus2, sus3, sus4},
		},
		{
			// Query for specific unit name should be fine
			"http://example.com/state?unitName=AAA",
			[]*schema.UnitState{sus1},
		},
		{
			// Query for a different specific unit name should be fine
			"http://example.com/state?unitName=CCC",
			[]*schema.UnitState{sus3, sus4},
		},
		{
			// Query for nonexistent unit name should return nothing
			"http://example.com/state?unitName=nope",
			nil,
		},
		{
			// Query for a specific machine ID should be fine
			"http://example.com/state?machineID=XXX",
			[]*schema.UnitState{sus2, sus3},
		},
		{
			// Query for nonexistent machine ID should return nothing
			"http://example.com/state?machineID=nope",
			nil,
		},
		{
			// Query for specific unit name and specific machine ID should filter by both
			"http://example.com/state?unitName=CCC&machineID=XXX",
			[]*schema.UnitState{sus3},
		},
	} {
		fr := registry.NewFakeRegistry()
		fr.SetUnitStates([]unit.UnitState{us1, us2, us3, us4})
		fAPI := &client.RegistryClient{Registry: fr}
		resource := &stateResource{fAPI, "/state", testTokenLimit}
		rw := httptest.NewRecorder()
		req, err := http.NewRequest("GET", tt.url, nil)
		if err != nil {
			t.Fatalf("case %d: Failed creating http.Request: %v", i, err)
		}

		resource.list(rw, req)
		if rw.Code != http.StatusOK {
			t.Errorf("case %d: Expected 200, got %d", i, rw.Code)
		}
		ct := rw.HeaderMap["Content-Type"]
		if len(ct) != 1 {
			t.Errorf("case %d: Response has wrong number of Content-Type values: %v", i, ct)
		} else if ct[0] != "application/json" {
			t.Errorf("case %d: Expected application/json, got %s", i, ct)
		}

		if rw.Body == nil {
			t.Errorf("case %d: Received nil response body", i)
			continue
		}

		var page schema.UnitStatePage
		if err := json.Unmarshal(rw.Body.Bytes(), &page); err != nil {
			t.Fatalf("case %d: Received unparseable body: %v", i, err)
		}

		got := page.States
		if !reflect.DeepEqual(got, tt.expected) {
			t.Errorf("case %d: Unexpected UnitStates received.", i)
			t.Logf("Got UnitStates:")
			for _, us := range got {
				t.Logf("%#v", us)
			}
			t.Logf("Expected UnitStates:")
			for _, us := range tt.expected {
				t.Logf("%#v", us)
			}

		}
	}

	fr := registry.NewFakeRegistry()
	fr.SetUnitStates([]unit.UnitState{
		unit.UnitState{UnitName: "XXX", ActiveState: "active"},
		unit.UnitState{UnitName: "YYY", ActiveState: "inactive"},
	})
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &stateResource{fAPI, "/state", testTokenLimit}
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

func TestUnitStateListBadNextPageToken(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &stateResource{fAPI, "/state", testTokenLimit}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/state?nextPageToken=EwBMLg==", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.list(rw, req)

	err = assertErrorResponse(rw, http.StatusBadRequest)
	if err != nil {
		t.Error(err)
	}
}

func TestExtractUnitStatePage(t *testing.T) {
	all := make([]*schema.UnitState, 103)
	for i := 0; i < 103; i++ {
		name := strconv.FormatInt(int64(i), 10)
		all[i] = &schema.UnitState{Name: name}
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
		items, next := extractUnitStatePageData(all, tt.token)
		expectCount := (tt.idxEnd - tt.idxStart + 1)
		if len(items) != expectCount {
			t.Errorf("case %d: expected page of %d, got %d", i, expectCount, len(items))
			continue
		}

		first := items[0].Name
		if first != strconv.FormatInt(int64(tt.idxStart), 10) {
			t.Errorf("case %d: first element in first page should have ID %d, got %s", i, tt.idxStart, first)
		}

		last := items[len(items)-1].Name
		if last != strconv.FormatInt(int64(tt.idxEnd), 10) {
			t.Errorf("case %d: first element in first page should have ID %d, got %s", i, tt.idxEnd, last)
		}

		if tt.next == nil && next != nil {
			t.Errorf("case %d: did not expect NextPageToken", i)
			continue
		} else if next == nil {
			if tt.next != nil {
				t.Errorf("case %d: did not receive expected NextPageToken", i)
			}
			continue
		}

		if !reflect.DeepEqual(next, tt.next) {
			t.Errorf("case %d: expected PageToken %v, got %v", i, tt.next, next)
		}
	}
}
