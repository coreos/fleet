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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func newUnit(t *testing.T, str string) unit.UnitFile {
	u, err := unit.NewUnitFile(str)
	if err != nil {
		t.Fatalf("Unexpected error creating unit from %q: %v", str, err)
	}
	return *u
}

func TestUnitsSubResourceNotFound(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fAPI := &client.RegistryClient{Registry: fr}
	ur := &unitsResource{fAPI, "/units"}
	rr := httptest.NewRecorder()

	req, err := http.NewRequest("GET", "/units/foo/bar", nil)
	if err != nil {
		t.Fatalf("Failed setting up http.Request for test: %v", err)
	}

	ur.ServeHTTP(rr, req)

	err = assertErrorResponse(rr, http.StatusNotFound)
	if err != nil {
		t.Error(err)
	}
}

func TestUnitsList(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fr.SetJobs([]job.Job{
		{Name: "XXX.service"},
		{Name: "YYY.service"},
	})
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &unitsResource{fAPI, "/units"}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/units", nil)
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
		var page schema.UnitPage
		err := json.Unmarshal(rw.Body.Bytes(), &page)
		if err != nil {
			t.Fatalf("Received unparseable body: %v", err)
		}

		if len(page.Units) != 2 || page.Units[0].Name != "XXX.service" || page.Units[1].Name != "YYY.service" {
			t.Errorf("Received incorrect UnitPage entity: %v", page)
		}
	}
}

func TestUnitsListBadNextPageToken(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &unitsResource{fAPI, "/units"}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/units?nextPageToken=EwBMLg==", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.list(rw, req)

	err = assertErrorResponse(rw, http.StatusBadRequest)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestExtractUnitPage(t *testing.T) {
	all := make([]*schema.Unit, 103)
	for i := 0; i < 103; i++ {
		name := strconv.FormatInt(int64(i), 10)
		all[i] = &schema.Unit{Name: name}
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
		items, next := extractUnitPageData(all, tt.token)
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

func TestUnitGet(t *testing.T) {
	tests := []struct {
		item string
		code int
	}{
		{item: "XXX.service", code: http.StatusOK},
		{item: "ZZZ", code: http.StatusNotFound},
	}

	fr := registry.NewFakeRegistry()
	fr.SetJobs([]job.Job{
		{Name: "XXX.service"},
		{Name: "YYY.service"},
	})
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &unitsResource{fAPI, "/units"}

	for i, tt := range tests {
		rw := httptest.NewRecorder()
		req, err := http.NewRequest("GET", fmt.Sprintf("http://example.com/units/%s", tt.item), nil)
		if err != nil {
			t.Errorf("case %d: failed creating http.Request: %v", i, err)
			continue
		}

		resource.get(rw, req, tt.item)

		if tt.code/100 == 2 {
			if tt.code != rw.Code {
				t.Errorf("case %d: expected %d, got %d", i, tt.code, rw.Code)
			}
		} else {
			err = assertErrorResponse(rw, tt.code)
			if err != nil {
				t.Errorf("case %d: %v", i, err)
			}
		}
	}
}

func TestUnitsDestroy(t *testing.T) {
	tests := []struct {
		// initial state of registry
		init []job.Job
		// name of unit to delete
		arg string
		// expected HTTP status code
		code int
		// expected state of registry after deletion attempt
		remaining []string
	}{
		// Deletion of an existing unit should succeed
		{
			init:      []job.Job{job.Job{Name: "XXX.service", Unit: newUnit(t, "[Service]\nFoo=Bar")}},
			arg:       "XXX.service",
			code:      http.StatusNoContent,
			remaining: []string{},
		},
		// Deletion of a nonexistent unit should fail
		{
			init:      []job.Job{job.Job{Name: "XXX.service", Unit: newUnit(t, "[Service]\nFoo=Bar")}},
			arg:       "YYY.service",
			code:      http.StatusNotFound,
			remaining: []string{"XXX.service"},
		},
	}

	for i, tt := range tests {
		fr := registry.NewFakeRegistry()
		fr.SetJobs(tt.init)

		req, err := http.NewRequest("DELETE", fmt.Sprintf("http://example.com/units/%s", tt.arg), nil)
		if err != nil {
			t.Errorf("case %d: failed creating http.Request: %v", i, err)
			continue
		}

		fAPI := &client.RegistryClient{Registry: fr}
		resource := &unitsResource{fAPI, "/units"}
		rw := httptest.NewRecorder()
		resource.destroy(rw, req, tt.arg)

		if tt.code/100 == 2 {
			if tt.code != rw.Code {
				t.Errorf("case %d: expected %d, got %d", i, tt.code, rw.Code)
			}
		} else {
			err = assertErrorResponse(rw, tt.code)
			if err != nil {
				t.Errorf("case %d: %v", i, err)
			}
		}

		units, err := fr.Units()
		if err != nil {
			t.Errorf("case %d: failed fetching Units after destruction: %v", i, err)
			continue
		}

		remaining := make([]string, len(units))
		for i, u := range units {
			remaining[i] = u.Name
		}

		if !reflect.DeepEqual(tt.remaining, remaining) {
			t.Errorf("case %d: expected Units %v, got %v", i, tt.remaining, remaining)
		}
	}
}

func TestUnitsSetDesiredState(t *testing.T) {
	tests := []struct {
		// initial state of Registry
		initJobs   []job.Job
		initStates map[string]job.JobState
		// item path (name) of the Unit
		item string
		// Unit to attempt to set
		arg schema.Unit
		// expected HTTP status code
		code int
		// expected state of registry after request
		finalStates map[string]job.JobState
	}{
		// Modify the desired State of an existing Job
		{
			initJobs:    []job.Job{job.Job{Name: "XXX.service", Unit: newUnit(t, "[Service]\nFoo=Bar")}},
			initStates:  map[string]job.JobState{"XXX.service": "inactive"},
			item:        "XXX.service",
			arg:         schema.Unit{Name: "XXX.service", DesiredState: "launched"},
			code:        http.StatusNoContent,
			finalStates: map[string]job.JobState{"XXX.service": "launched"},
		},
		// Create a new Unit
		{
			initJobs:   []job.Job{},
			initStates: map[string]job.JobState{},
			item:       "YYY.service",
			arg: schema.Unit{
				Name:         "YYY.service",
				DesiredState: "loaded",
				Options: []*schema.UnitOption{
					&schema.UnitOption{Section: "Service", Name: "Foo", Value: "Baz"},
				},
			},
			code:        http.StatusCreated,
			finalStates: map[string]job.JobState{"YYY.service": "loaded"},
		},
		// Creating a new Unit without Options fails
		{
			initJobs:   []job.Job{},
			initStates: map[string]job.JobState{},
			item:       "YYY.service",
			arg: schema.Unit{
				Name:         "YYY.service",
				DesiredState: "loaded",
				Options:      []*schema.UnitOption{},
			},
			code:        http.StatusConflict,
			finalStates: map[string]job.JobState{},
		},
		// Referencing a Unit where the name is inconsistent with the path should fail
		{
			initJobs: []job.Job{
				job.Job{Name: "XXX.service", Unit: newUnit(t, "[Service]\nFoo=Bar")},
				job.Job{Name: "YYY.service", Unit: newUnit(t, "[Service]\nFoo=Baz")},
			},
			initStates: map[string]job.JobState{
				"XXX.service": "inactive",
				"YYY.service": "inactive",
			},
			item: "XXX.service",
			arg: schema.Unit{
				Name:         "YYY.service",
				DesiredState: "loaded",
			},
			code: http.StatusBadRequest,
			finalStates: map[string]job.JobState{
				"XXX.service": "inactive",
				"YYY.service": "inactive",
			},
		},
		// Referencing a Unit where the name is omitted should substitute the name from the path
		{
			initJobs: []job.Job{
				job.Job{Name: "XXX.service", Unit: newUnit(t, "[Service]\nFoo=Bar")},
				job.Job{Name: "YYY.service", Unit: newUnit(t, "[Service]\nFoo=Baz")},
			},
			initStates: map[string]job.JobState{
				"XXX.service": "inactive",
				"YYY.service": "inactive",
			},
			item: "XXX.service",
			arg: schema.Unit{
				DesiredState: "loaded",
			},
			code: http.StatusNoContent,
			finalStates: map[string]job.JobState{
				"XXX.service": "loaded",
				"YYY.service": "inactive",
			},
		},
	}

	for i, tt := range tests {
		fr := registry.NewFakeRegistry()
		fr.SetJobs(tt.initJobs)
		for j, s := range tt.initStates {
			err := fr.SetUnitTargetState(j, s)
			if err != nil {
				t.Errorf("case %d: failed initializing unit's target state: %v", i, err)
			}
		}

		req, err := http.NewRequest("PUT", fmt.Sprintf("http://example.com/units/%s", tt.item), nil)
		if err != nil {
			t.Errorf("case %d: failed creating http.Request: %v", i, err)
			continue
		}

		enc, err := json.Marshal(tt.arg)
		if err != nil {
			t.Errorf("case %d: unable to JSON-encode request: %v", i, err)
			continue
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(enc))
		req.Header.Set("Content-Type", "application/json")

		fAPI := &client.RegistryClient{Registry: fr}
		resource := &unitsResource{fAPI, "/units"}
		rw := httptest.NewRecorder()
		resource.set(rw, req, tt.item)

		if tt.code/100 == 2 {
			if tt.code != rw.Code {
				t.Errorf("case %d: expected %d, got %d", i, tt.code, rw.Code)
			}
		} else {
			err = assertErrorResponse(rw, tt.code)
			if err != nil {
				t.Errorf("case %d: %v", i, err)
			}
		}

		for name, expect := range tt.finalStates {
			u, err := fr.Unit(name)
			if err != nil {
				t.Errorf("case %d: failed fetching Job: %v", i, err)
			} else if u == nil {
				t.Errorf("case %d: fetched nil Unit(%s), expected non-nil", i, name)
				continue
			}

			if u.TargetState != expect {
				t.Errorf("case %d: expect Unit(%s) target state %q, got %q", i, name, expect, u.TargetState)
			}
		}
	}
}

func makeConflictUO(name string) *schema.UnitOption {
	return &schema.UnitOption{
		Section: "X-Fleet",
		Name:    "Conflicts",
		Value:   name,
	}
}

func makePeerUO(name string) *schema.UnitOption {
	return &schema.UnitOption{
		Section: "X-Fleet",
		Name:    "MachineOf",
		Value:   name,
	}
}

func makeIDUO(name string) *schema.UnitOption {
	return &schema.UnitOption{
		Section: "X-Fleet",
		Name:    "MachineID",
		Value:   name,
	}
}

func TestValidateOptions(t *testing.T) {
	testCases := []struct {
		opts  []*schema.UnitOption
		valid bool
	}{
		// Empty set is fine
		{
			nil,
			true,
		},
		{
			[]*schema.UnitOption{},
			true,
		},
		// Non-overlapping peers/conflicts are fine
		{
			[]*schema.UnitOption{
				makeConflictUO("foo.service"),
				makeConflictUO("bar.service"),
			},
			true,
		},
		{
			[]*schema.UnitOption{
				makeConflictUO("foo.service"),
				makePeerUO("bar.service"),
			},
			true,
		},
		{
			[]*schema.UnitOption{
				makeConflictUO("foo.service"),
				makePeerUO("bar.service"),
			},
			true,
		},
		{
			[]*schema.UnitOption{
				makeConflictUO("b*e"),
				makePeerUO("foo.service"),
			},
			true,
		},
		// Intersecting peers/conflicts are no good
		{
			[]*schema.UnitOption{
				makeConflictUO("foo.service"),
				makePeerUO("foo.service"),
			},
			false,
		},
		{
			[]*schema.UnitOption{
				makeConflictUO("foo.service"),
				makeConflictUO("bar.service"),
				makePeerUO("bar.service"),
			},
			false,
		},
		{
			[]*schema.UnitOption{
				makeConflictUO("b*e"),
				makePeerUO("bar.service"),
			},
			false,
		},
		{
			[]*schema.UnitOption{
				makeConflictUO("b*e"),
				makePeerUO("baz.service"),
			},
			false,
		},
		// MachineID is fine by itself
		{
			[]*schema.UnitOption{
				makeIDUO("abcdefghi"),
			},
			true,
		},
		// MachineID with Peers no good
		{
			[]*schema.UnitOption{
				makeIDUO("abcdefghi"),
				makePeerUO("foo.service"),
			},
			false,
		},
		{
			[]*schema.UnitOption{
				makeIDUO("zyxwvutsr"),
				makePeerUO("bar.service"),
				makePeerUO("foo.service"),
			},
			false,
		},
		// MachineID with Conflicts no good
		{
			[]*schema.UnitOption{
				makeIDUO("abcdefghi"),
				makeConflictUO("bar.service"),
			},
			false,
		},
		{
			[]*schema.UnitOption{
				makeIDUO("zyxwvutsr"), makeConflictUO("foo.service"), makeConflictUO("bar.service"),
			},
			false,
		},
		// Global by itself is OK
		{
			[]*schema.UnitOption{
				&schema.UnitOption{
					Section: "X-Fleet",
					Name:    "Global",
					Value:   "true",
				},
			},
			true,
		},
		// Global with Conflicts is ok
		{
			[]*schema.UnitOption{
				&schema.UnitOption{
					Section: "X-Fleet",
					Name:    "Global",
					Value:   "true",
				},
				makeConflictUO("foo.service"),
			},
			true,
		},
		{
			[]*schema.UnitOption{
				&schema.UnitOption{
					Section: "X-Fleet",
					Name:    "Global",
					Value:   "true",
				},
				makeConflictUO("bar.service"),
			},
			true,
		},
		// Global with peer no good
		{
			[]*schema.UnitOption{
				&schema.UnitOption{
					Section: "X-Fleet",
					Name:    "Global",
					Value:   "true",
				},
				makePeerUO("foo.service"),
				makePeerUO("bar.service"),
			},
			false,
		},
		// Global with MachineID no good
		{
			[]*schema.UnitOption{
				&schema.UnitOption{
					Section: "X-Fleet",
					Name:    "Global",
					Value:   "true",
				},
				makeIDUO("abcdefghi"),
			},
			false,
		},
		{
			[]*schema.UnitOption{
				makeIDUO("abcdefghi"),
				&schema.UnitOption{
					Section: "X-Fleet",
					Name:    "Global",
					Value:   "true",
				},
			},
			false,
		},
	}
	for i, tt := range testCases {
		err := ValidateOptions(tt.opts)
		if (err == nil) != tt.valid {
			t.Errorf("case %d: bad error value (got err=%v, want valid=%t)", i, err, tt.valid)
		}
	}
}

func TestValidateName(t *testing.T) {
	badTestCases := []string{
		// cannot be empty
		"",
		// cannot be longer than unitNameMax
		fmt.Sprintf("%0"+strconv.Itoa(unitNameMax+1)+"s", ".service"),
		fmt.Sprintf("%0"+strconv.Itoa(unitNameMax*2)+"s", ".mount"),
		// must contain "."
		"fooservice",
		"barmount",
		"bar@foo",
		// cannot end in "."
		"foo.",
		"foo.service.",
		"foo@foo.service.",
		// must have valid unit suffix
		"foo.bar",
		"hello.cerveza",
		"foo.servICE",
		// cannot have invalid characters
		"foo%.service",
		"foo$asd.service",
		"hello##.mount",
		"yes/no.service",
		"this+that.mount",
		"dog=woof@.mount",
		// cannot start in "@"
		"@foo.service",
		"@this.mount",
	}
	for _, name := range badTestCases {
		if err := ValidateName(name); err == nil {
			t.Errorf("name %q: validation did not fail as expected!", name)
		}
	}

	goodTestCases := []string{
		"foo.service",
		"hello.mount",
		"foo@123.service",
		"foo@.service",
		"yo.yo.service",
		"hello@world.path",
		"hello:world.service",
		"yes@no\\.service",
		"foo-bar.mount",
		"jalapano_chips.service",
		// generate a name the exact length of unitNameMax
		fmt.Sprintf("%0"+strconv.Itoa(unitNameMax)+"s", ".service"),
	}
	for _, name := range goodTestCases {
		if err := ValidateName(name); err != nil {
			t.Errorf("name %q: validation failed unexpectedly! err=%v", name, err)
		}
	}
}

func TestUnitsSetDesiredStateBadContentType(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fAPI := &client.RegistryClient{Registry: fr}
	resource := &unitsResource{fAPI, "/units"}
	rr := httptest.NewRecorder()

	body := ioutil.NopCloser(bytes.NewBuffer([]byte(`{"foo":"bar"}`)))
	req, err := http.NewRequest("PUT", "http://example.com/units/foo.service", body)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	req.Header.Set("Content-Type", "application/xml")

	resource.set(rr, req, "foo.service")

	err = assertErrorResponse(rr, http.StatusUnsupportedMediaType)
	if err != nil {
		t.Fatal(err)
	}
}
