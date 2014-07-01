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
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

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
			t.Errorf("case %d: call to extractUnitPage failed: %v", err)
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
				Unit:            unit.Unit{Raw: "[Service]\nExecStart=/usr/bin/sleep 3000\n"},
				UnitState: &unit.UnitState{
					LoadState:    "loaded",
					ActiveState:  "active",
					SubState:     "running",
					MachineState: &machine.MachineState{ID: "YYY"},
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
				FileContents: "W1NlcnZpY2VdCkV4ZWNTdGFydD0vdXNyL2Jpbi9zbGVlcCAzMDAwCg==",
				FileHash:     "248b997d6becee1b835b7ec7d9c8e68d7dd24623",
			},
		},
	}

	for i, tt := range tests {
		output, err := mapJobToSchema(&tt.input)
		if err != nil {
			t.Errorf("case %d: call to mapJobToSchema failed: %v", err)
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
		// which Job to attempt to delete
		arg schema.DeletableUnit
		// expected HTTP status code
		code int
		// expected state of registry after deletion attempt
		remaining []string
	}{
		// Unsafe deletion of an existing unit should succeed
		{
			init:      []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			arg:       schema.DeletableUnit{Name: "XXX"},
			code:      http.StatusNoContent,
			remaining: []string{},
		},
		// Safe deletion of an existing unit should succeed
		{
			init:      []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			arg:       schema.DeletableUnit{Name: "XXX", FileContents: "Rk9P"},
			code:      http.StatusNoContent,
			remaining: []string{},
		},
		// Unsafe deletion of a nonexistent unit should fail
		{
			init:      []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			arg:       schema.DeletableUnit{Name: "YYY"},
			code:      http.StatusNotFound,
			remaining: []string{"XXX"},
		},
		// Safe deletion of a nonexistent unit should fail
		{
			init:      []job.Job{},
			arg:       schema.DeletableUnit{Name: "XXX", FileContents: "Rk9P"},
			code:      http.StatusNotFound,
			remaining: []string{},
		},
		// Safe deletion of a unit with the wrong contents should fail
		{
			init:      []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			arg:       schema.DeletableUnit{Name: "XXX", FileContents: "QkFS"},
			code:      http.StatusConflict,
			remaining: []string{"XXX"},
		},
		// Safe deletion of a unit with the malformed contents should fail
		{
			init:      []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			arg:       schema.DeletableUnit{Name: "XXX", FileContents: "*"},
			code:      http.StatusBadRequest,
			remaining: []string{"XXX"},
		},
	}

	for i, tt := range tests {
		fr := registry.NewFakeRegistry()
		fr.SetJobs(tt.init)

		req, err := http.NewRequest("DELETE", fmt.Sprintf("http://example.com/units/%s", tt.arg.Name), nil)
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
		resource.destroy(rw, req, tt.arg.Name)

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

		jobs, err := fr.Jobs()
		if err != nil {
			t.Errorf("case %d: failed fetching Jobs after destruction: %v", i, err)
			continue
		}

		remaining := make([]string, len(jobs))
		for i, j := range jobs {
			remaining[i] = j.Name
		}

		if !reflect.DeepEqual(tt.remaining, remaining) {
			t.Errorf("case %d: expected Jobs %v, got %v", i, tt.remaining, remaining)
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
			initJobs:    []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			initStates:  map[string]job.JobState{"XXX": "inactive"},
			arg:         schema.DesiredUnitState{Name: "XXX", DesiredState: "launched"},
			code:        http.StatusNoContent,
			finalStates: map[string]job.JobState{"XXX": "launched"},
		},
		// Create a new Job
		{
			initJobs:    []job.Job{},
			initStates:  map[string]job.JobState{},
			arg:         schema.DesiredUnitState{Name: "YYY", DesiredState: "loaded", FileContents: "cGVubnkNCg=="},
			code:        http.StatusNoContent,
			finalStates: map[string]job.JobState{"YYY": "loaded"},
		},
		{
			initJobs:    []job.Job{},
			initStates:  map[string]job.JobState{},
			arg:         schema.DesiredUnitState{Name: "YYY", DesiredState: "loaded", FileContents: "*"},
			code:        http.StatusBadRequest,
			finalStates: map[string]job.JobState{},
		},
		// Modifying a Job with garbage fileContents should fail
		{
			initJobs:    []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			initStates:  map[string]job.JobState{"XXX": job.JobStateInactive},
			arg:         schema.DesiredUnitState{Name: "YYY", DesiredState: "loaded", FileContents: "*"},
			code:        http.StatusBadRequest,
			finalStates: map[string]job.JobState{"XXX": job.JobStateInactive},
		},
		// Modifying a nonexistent Job should fail
		{
			initJobs:    []job.Job{},
			initStates:  map[string]job.JobState{},
			arg:         schema.DesiredUnitState{Name: "YYY", DesiredState: "loaded"},
			code:        http.StatusConflict,
			finalStates: map[string]job.JobState{},
		},
		// Modifying a Job with the incorrect fileContents should fail
		{
			initJobs:    []job.Job{job.Job{Name: "XXX", Unit: unit.Unit{Raw: "FOO"}}},
			initStates:  map[string]job.JobState{"XXX": "inactive"},
			arg:         schema.DesiredUnitState{Name: "XXX", DesiredState: "loaded", FileContents: "ZWxyb3kNCg=="},
			code:        http.StatusConflict,
			finalStates: map[string]job.JobState{"XXX": "inactive"},
		},
	}

	for i, tt := range tests {
		fr := registry.NewFakeRegistry()
		fr.SetJobs(tt.initJobs)
		for j, s := range tt.initStates {
			err := fr.SetJobTargetState(j, s)
			if err != nil {
				t.Errorf("case %d: failed initializing Job target state: %v", err)
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
			j, err := fr.Job(name)
			if err != nil {
				t.Errorf("case %d: failed fetching Job: %v", i, err)
			} else if j == nil {
				t.Errorf("case %d: fetched nil Job(%s), expected non-nil", i, name)
			}

			if j.TargetState != expect {
				t.Errorf("case %d: expect Job(%s) target state %q, got %q", i, name, expect, j.TargetState)
			}
		}
	}
}
