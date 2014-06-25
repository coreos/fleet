package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

func TestMachinesList(t *testing.T) {
	fr := registry.NewFakeRegistry()
	fr.SetMachines([]machine.MachineState{
		{ID: "XXX", PublicIP: "", Metadata: nil},
		{ID: "YYY", PublicIP: "1.2.3.4", Metadata: map[string]string{"ping": "pong"}},
	})
	resource := &machinesResource{fr}
	rw := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "http://example.com", nil)
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
		expected := `{"data":{"items":[{"id":"XXX"},{"id":"YYY","metadata":{"ping":"pong"},"primaryIP":"1.2.3.4"}]}}`
		if body != expected {
			t.Errorf("Expected body:\n%s\n\nReceived body:\n%s\n", expected, body)
		}
	}
}

func TestMachinesListBadNextPageToken(t *testing.T) {
	fr := registry.NewFakeRegistry()
	resource := &machinesResource{fr}
	rw := httptest.NewRecorder()
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
		resp := extractMachinePage(all, tt.token)
		expectCount := (tt.idxEnd - tt.idxStart + 1)
		if len(resp.Data.Items) != expectCount {
			t.Fatalf("case %d: expected page of %d, got %d", i, expectCount, len(resp.Data.Items))
		}

		first := resp.Data.Items[0].Id
		if first != strconv.FormatInt(int64(tt.idxStart), 10) {
			t.Errorf("case %d: first element in page should have ID %d, got %d", i, tt.idxStart, first)
		}

		last := resp.Data.Items[len(resp.Data.Items)-1].Id
		if last != strconv.FormatInt(int64(tt.idxEnd), 10) {
			t.Errorf("case %d: first element in page should have ID %d, got %d", i, tt.idxEnd, last)
		}

		if tt.next == nil && resp.Data.NextPageToken != "" {
			t.Errorf("case %d: did not expect NextPageToken", i)
			continue
		} else if resp.Data.NextPageToken == "" {
			if tt.next != nil {
				t.Errorf("case %d: did not receive expected NextPageToken", i)
			}
			continue
		}

		next, err := decodePageToken(resp.Data.NextPageToken)
		if err != nil {
			t.Errorf("case %d: unable to parse NextPageToken: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(next, tt.next) {
			t.Errorf("case %d: expected PageToken %v, got %v", i, tt.next, next)
		}
	}
}
