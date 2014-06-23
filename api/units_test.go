package api

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
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
	if rw.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rw.Code)
	}

	if rw.Body.Len() != 0 {
		t.Error("Received non-empty response body")
	}
}

func TestExtractUnitPage(t *testing.T) {
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
		page := extractUnitPage(all, tt.token)
		expectCount := (tt.idxEnd - tt.idxStart + 1)
		if len(page.Units) != expectCount {
			t.Fatalf("case %d: expected page of %d, got %d", i, expectCount, len(page.Units))
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
				Name:     "XXX",
				State:    &loaded,
				Unit:     unit.Unit{Raw: "[Service]\nExecStart=/usr/bin/sleep 3000\n"},
				UnitHash: unit.Hash([sha1.Size]byte{36, 139, 153, 125, 107, 236, 238, 27, 131, 91, 126, 199, 217, 200, 230, 141, 125, 210, 70, 35}),
				UnitState: &unit.UnitState{
					LoadState:    "loaded",
					ActiveState:  "active",
					SubState:     "running",
					MachineState: &machine.MachineState{ID: "YYY"},
				},
			},
			schema.Unit{
				Name:         "XXX",
				CurrentState: "loaded",
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
		output := mapJobToSchema(&tt.input)
		if !reflect.DeepEqual(tt.expect, *output) {
			t.Errorf("case %d: expect=%v, got=%v", i, tt.expect, *output)
		}
	}
}

func TestUnitGet(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fr.SetJobs([]job.Job{
		{Name: "XXX"},
		{Name: "YYY"},
	})
	resource := &unitsResource{fr, "/units"}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com/units/XXX", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	resource.get(rw, req, "XXX")
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
		var unit schema.Unit
		err := json.Unmarshal(rw.Body.Bytes(), &unit)
		if err != nil {
			t.Fatalf("Received unparseable body: %v", err)
		}

		if unit.Name != "XXX" {
			t.Errorf("Received incorrect Unit entity: %v", unit)
		}
	}
}

func TestUnitsDestroy(t *testing.T) {
	tests := []struct {
		// Starting point for the Registry
		fixtures []job.Job
		// Actual request being submtited
		argument schema.DeletableUnitCollection

		// Expected HTTP response code
		code int
		// Expected Jobs left in the Registry
		remaining []string
	}{
		// Deleting a single Job should succeed
		{
			fixtures: []job.Job{
				{Name: "XXX", Unit: unit.Unit{Raw: "ALPHA"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "XXX"},
				},
			},
			code:      http.StatusNoContent,
			remaining: []string{},
		},

		// Deleting multiple Jobs should succeed
		{
			fixtures: []job.Job{
				{Name: "XXX", Unit: unit.Unit{Raw: "ALPHA"}},
				{Name: "YYY", Unit: unit.Unit{Raw: "BRAVO"}},
				{Name: "ZZZ", Unit: unit.Unit{Raw: "CHARLIE"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "XXX"},
					{Name: "YYY"},
					{Name: "ZZZ"},
				},
			},
			code:      http.StatusNoContent,
			remaining: []string{},
		},

		// Attempting to delete nonexistent Job should not fail
		{
			fixtures: []job.Job{
				{Name: "XXX", Unit: unit.Unit{Raw: "ALPHA"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "ZZZ"},
				},
			},
			code:      http.StatusNoContent,
			remaining: []string{"XXX"},
		},

		// Deleting a single Job should not affect other Jobs
		{
			fixtures: []job.Job{
				{Name: "XXX", Unit: unit.Unit{Raw: "ALPHA"}},
				{Name: "YYY", Unit: unit.Unit{Raw: "BRAVO"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "XXX"},
				},
			},
			code:      http.StatusNoContent,
			remaining: []string{"YYY"},
		},

		// Deleting a single Job with the correct FileHash should succeed
		{
			fixtures: []job.Job{
				{Name: "ZZZ", Unit: unit.Unit{Raw: "CHARLIE"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "ZZZ", FileHash: "70f91352865ca41f8cffbeff845a847192a1e7d3"},
				},
			},
			code:      http.StatusNoContent,
			remaining: []string{},
		},

		// Deleting a single Job with the incorrect FileHash should fail
		{
			fixtures: []job.Job{
				{Name: "ZZZ", Unit: unit.Unit{Raw: "CHARLIE"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "ZZZ", FileHash: "b10fe0fb7ff3dc28d35773e0c171046c883572eb"},
				},
			},
			code:      http.StatusConflict,
			remaining: []string{"ZZZ"},
		},

		// Deleting a single Job with the incorrect FileHash should fail, but not block other delete operations
		{
			fixtures: []job.Job{
				{Name: "XXX", Unit: unit.Unit{Raw: "ALPHA"}},
				{Name: "YYY", Unit: unit.Unit{Raw: "BRAVO"}},
				{Name: "ZZZ", Unit: unit.Unit{Raw: "CHARLIE"}},
			},
			argument: schema.DeletableUnitCollection{
				Units: []*schema.DeletableUnit{
					{Name: "XXX"},
					{Name: "YYY", FileHash: "70f91352865ca41f8cffbeff845a847192a1e7d3"},
					{Name: "ZZZ"},
				},
			},
			code:      http.StatusConflict,
			remaining: []string{"YYY"},
		},
	}

	for i, tt := range tests {
		fr := registry.NewFakeRegistry()
		fr.SetJobs(tt.fixtures)
		resource := &unitsResource{fr, "/units"}
		rw := httptest.NewRecorder()

		req, err := http.NewRequest("DELETE", "http://example.com/units", nil)
		if err != nil {
			t.Errorf("case %d: failed creating http.Request: %v", i, err)
			continue
		}

		enc, err := json.Marshal(tt.argument)
		if err != nil {
			t.Errorf("case %d: unable to JSON-encode request: %v", i, err)
			continue
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(enc))
		req.Header.Set("Content-Type", "application/json")

		resource.destroy(rw, req)
		if rw.Code != tt.code {
			t.Errorf("case %d: expected %d, got %d", i, tt.code, rw.Code)
		}

		jobs, err := fr.Jobs()
		if err != nil {
			t.Errorf("case %d: failed fetching list of Jobs after test: %v", i, err)
			continue
		}

		remaining := make([]string, len(jobs))
		for i, j := range jobs {
			remaining[i] = j.Name
		}
		if !reflect.DeepEqual(remaining, tt.remaining) {
			t.Errorf("case %d: expected remaining jobs %v, got %v", i, tt.remaining, remaining)
		}
	}
}

func TestUnitsSetDesiredUnitStates(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fr.SetJobs([]job.Job{
		{Name: "XXX"},
		{Name: "YYY"},
	})
	fr.SetJobTargetState("XXX", "active")
	fr.SetJobTargetState("YYY", "inactive")

	resource := &unitsResource{fr, "/units"}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed creating http.Request: %v", err)
	}

	body := schema.DesiredUnitStateCollection{
		Units: []*schema.DesiredUnitState{
			{Name: "XXX", DesiredState: "loaded"},
			{Name: "YYY", DesiredState: "launched"},
			{Name: "ZZZ", DesiredState: "loaded", FileContents: "W1NlcnZpY2VdCkV4ZWNTdGFydD0vdXNyL2Jpbi9zbGVlcCAzMDAwCg=="},
		},
	}
	enc, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Unable to JSON-encode request: %v", err)
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(enc))
	req.Header.Set("Content-Type", "application/json")

	resource.set(rw, req)
	if rw.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rw.Code)
	}

	expect := map[string]string{
		"XXX": "loaded",
		"YYY": "launched",
		"ZZZ": "loaded",
	}
	for name, state := range expect {
		j, _ := fr.Job(name)
		if j == nil {
			t.Errorf("Expected job %s to exist, got nil", j.Name)
			continue
		}

		ts, _ := fr.JobTargetState(name)
		if ts == nil {
			t.Errorf("Expected job %s to have target state %v, got nil", j, state)
		} else if sts := string(*ts); sts != state {
			t.Errorf("Expected job %s to have target state %v, got %v", j, state, sts)
		}
	}
}
