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
	ur := &unitsResource{fr, "/units"}
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
		{Name: "XXX"},
		{Name: "YYY"},
	})
	resource := &unitsResource{fr, "/units"}
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

		if len(page.Units) != 2 || page.Units[0].Name != "XXX" || page.Units[1].Name != "YYY" {
			t.Errorf("Received incorrect UnitPage entity: %v", page)
		}
	}
}

func TestUnitsListBadNextPageToken(t *testing.T) {
	fr := registry.NewFakeRegistry()
	resource := &unitsResource{fr, "/units"}
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
	fr := registry.NewFakeRegistry()

	all := make([]job.Job, 103)
	for i := 0; i < 103; i++ {
		name := strconv.FormatInt(int64(i), 10)
		all[i] = job.Job{Name: name}
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
		page, err := extractUnitPage(fr, all, tt.token)
		if err != nil {
			t.Errorf("case %d: call to extractUnitPage failed: %v", i, err)
			continue
		}
		expectCount := (tt.idxEnd - tt.idxStart + 1)
		if len(page.Units) != expectCount {
			t.Errorf("case %d: expected page of %d, got %d", i, expectCount, len(page.Units))
			continue
		}

		first := page.Units[0].Name
		if first != strconv.FormatInt(int64(tt.idxStart), 10) {
			t.Errorf("case %d: irst element in first page should have ID %d, got %d", i, tt.idxStart, first)
		}

		last := page.Units[len(page.Units)-1].Name
		if last != strconv.FormatInt(int64(tt.idxEnd), 10) {
			t.Errorf("case %d: first element in first page should have ID %d, got %d", i, tt.idxEnd, last)
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

func TestMapJobToSchema(t *testing.T) {
	loaded := job.JobStateLoaded

	tests := []struct {
		input  job.Job
		expect schema.Unit
	}{
		{
			job.Job{
				Name:            "XXX",
				State:           &loaded,
				TargetState:     job.JobStateLaunched,
				TargetMachineID: "ZZZ",
				Unit:            newUnit(t, "[Service]\nExecStart=/usr/bin/sleep 3000\n"),
				UnitState: &unit.UnitState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "YYY",
				},
			},
			schema.Unit{
				Name:            "XXX",
				CurrentState:    "loaded",
				DesiredState:    "launched",
				TargetMachineID: "ZZZ",
				Systemd: &schema.SystemdState{
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
					MachineID:   "YYY",
				},
				Options: []*schema.UnitOption{
					&schema.UnitOption{Section: "Service", Name: "ExecStart", Value: "/usr/bin/sleep 3000"},
				},
			},
		},
	}

	for i, tt := range tests {
		output, err := mapJobToSchema(&tt.input)
		if err != nil {
			t.Errorf("case %d: call to mapJobToSchema failed: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expect, *output) {
			t.Errorf("case %d: expect=%v, got=%v", i, tt.expect, *output)
		}
	}
}

func TestUnitGet(t *testing.T) {
	tests := []struct {
		item string
		code int
	}{
		{item: "XXX", code: http.StatusOK},
		{item: "ZZZ", code: http.StatusNotFound},
	}

	fr := registry.NewFakeRegistry()
	fr.SetJobs([]job.Job{
		{Name: "XXX"},
		{Name: "YYY"},
	})
	resource := &unitsResource{fr, "/units"}

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
			init:      []job.Job{job.Job{Name: "XXX", Unit: newUnit(t, "[Service]\nFoo=Bar")}},
			arg:       "XXX",
			code:      http.StatusNoContent,
			remaining: []string{},
		},
		// Deletion of a nonexistent unit should fail
		{
			init:      []job.Job{job.Job{Name: "XXX", Unit: newUnit(t, "[Service]\nFoo=Bar")}},
			arg:       "YYY",
			code:      http.StatusNotFound,
			remaining: []string{"XXX"},
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

		resource := &unitsResource{fr, "/units"}
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
		// which Job to attempt to delete
		arg schema.DesiredUnitState
		// expected HTTP status code
		code int
		// expected state of registry after request
		finalStates map[string]job.JobState
	}{
		// Modify the DesiredState of an existing Job
		{
			initJobs:    []job.Job{job.Job{Name: "XXX", Unit: newUnit(t, "[Service]\nFoo=Bar")}},
			initStates:  map[string]job.JobState{"XXX": "inactive"},
			arg:         schema.DesiredUnitState{Name: "XXX", DesiredState: "launched"},
			code:        http.StatusNoContent,
			finalStates: map[string]job.JobState{"XXX": "launched"},
		},
		// Create a new Job
		{
			initJobs:   []job.Job{},
			initStates: map[string]job.JobState{},
			arg: schema.DesiredUnitState{
				Name:         "YYY",
				DesiredState: "loaded",
				Options: []*schema.UnitOption{
					&schema.UnitOption{Section: "Service", Name: "Foo", Value: "Baz"},
				},
			},
			code:        http.StatusNoContent,
			finalStates: map[string]job.JobState{"YYY": "loaded"},
		},
		// Creating a new Job without Options fails
		{
			initJobs:   []job.Job{},
			initStates: map[string]job.JobState{},
			arg: schema.DesiredUnitState{
				Name:         "YYY",
				DesiredState: "loaded",
				Options:      []*schema.UnitOption{},
			},
			code:        http.StatusConflict,
			finalStates: map[string]job.JobState{},
		},
		// Modifying a nonexistent Job should fail
		{
			initJobs:    []job.Job{},
			initStates:  map[string]job.JobState{},
			arg:         schema.DesiredUnitState{Name: "YYY", DesiredState: "loaded"},
			code:        http.StatusConflict,
			finalStates: map[string]job.JobState{},
		},
	}

	for i, tt := range tests {
		if i != 1 {
			continue
		}
		fr := registry.NewFakeRegistry()
		fr.SetJobs(tt.initJobs)
		for j, s := range tt.initStates {
			err := fr.SetUnitTargetState(j, s)
			if err != nil {
				t.Errorf("case %d: failed initializing unit's target state: %v", i, err)
			}
		}

		req, err := http.NewRequest("PUT", fmt.Sprintf("http://example.com/units/%s", tt.arg.Name), nil)
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

		resource := &unitsResource{fr, "/units"}
		rw := httptest.NewRecorder()
		resource.set(rw, req, tt.arg.Name)

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
