// Copyright 2016 The fleet Authors
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

package functional

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// schemaMachines structures should match with those of
// schema/v1-json.go. Note that these structures cannot be
// directly imported from schema, because functional tests
// would then fail to compile.
type fxSchemaMachine struct {
	Id string `json:"id,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`

	PrimaryIP string `json:"primaryIP,omitempty"`
}

type fxSchemaMachinePage struct {
	Machines []*fxSchemaMachine `json:"machines,omitempty"`

	NextPageToken string `json:"nextPageToken,omitempty"`
}

// machineMetadataOp structure should match with those of
// api/machines.go. Note that these structures cannot be
// directly imported from api, because functional tests
// would then fail to compile.
type ValueType struct {
	Value string `json:"value"`
}

type machineMetadataOp struct {
	Operation string `json:"op"`
	Path      string `json:"path"`
	Value     ValueType
}

func TestMachinesList(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	m, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForNMachines(m, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Get a normal machine list, should return OK
	resp, err := getHTTPResponse("GET", m.Endpoint()+"/fleet/v1/machines", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Got HTTP response status %d.", resp.StatusCode)
	}

	err = checkContentType(resp)
	if err != nil {
		t.Fatal(err)
	}

	testMachinesListBadNextPageToken(t, m)
}

func testMachinesListBadNextPageToken(t *testing.T, m platform.Member) {
	// Send an invalid GET request, should return failure
	resp, err := getHTTPResponse("GET", m.Endpoint()+"/fleet/v1/machines?nextPageToken=EwBMLg==", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d.", http.StatusBadRequest, resp.StatusCode)
	}

	err = checkContentType(resp)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMachinesPatchAddModify(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	m1, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForNMachines(m0, 2)
	if err != nil {
		t.Fatal(err)
	}

	sm0 := &fxSchemaMachine{
		Id: m0.ID(),
		Metadata: map[string]string{
			"foo":      "bar",
			"hostname": fmt.Sprintf("smoke%s", m0.ID()),
		},
		PrimaryIP: m0.IP(),
	}

	sm1 := &fxSchemaMachine{
		Id: m1.ID(),
		Metadata: map[string]string{
			"ping":     "splat",
			"hostname": fmt.Sprintf("smoke%s", m1.ID()),
		},
		PrimaryIP: m1.IP(),
	}

	msp0 := &machineMetadataOp{
		Operation: "add",
		Path:      fmt.Sprintf("/%s/metadata/foo", m0.ID()),
		Value:     ValueType{Value: "bar"},
	}
	msp1 := &machineMetadataOp{
		Operation: "replace",
		Path:      fmt.Sprintf("/%s/metadata/ping", m1.ID()),
		Value:     ValueType{Value: "splat"},
	}
	msreq, err := json.Marshal([]*machineMetadataOp{msp0, msp1})
	if err != nil {
		t.Fatalf("unexpected error marshalling: %#v", err)
	}
	reqBody := string(msreq)

	respPatch, err := getHTTPResponse("PATCH", m0.Endpoint()+"/fleet/v1/machines", reqBody)
	if err != nil {
		t.Fatal(err)
	}
	defer respPatch.Body.Close()

	if respPatch.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected status %d, got %d.", http.StatusNoContent, respPatch.StatusCode)
	}

	err = checkContentType(respPatch)
	if err != nil {
		t.Fatal(err)
	}

	// Send a normal GET to get list to be compared to an expected list
	respGet, err := getHTTPResponse("GET", m0.Endpoint()+"/fleet/v1/machines", "")
	if err != nil {
		t.Fatal(err)
	}
	defer respGet.Body.Close()

	if respGet.StatusCode != http.StatusOK {
		t.Fatalf("Got HTTP response status %d.", respGet.StatusCode)
	}

	err = checkContentType(respGet)
	if err != nil {
		t.Fatal(err)
	}

	body, rerr := ioutil.ReadAll(respGet.Body)
	if rerr != nil {
		t.Fatalf("Failed to read response body: %v", rerr)
	}

	// unmarshal, compare
	var page fxSchemaMachinePage
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("Received unparsable body: %v", err)
	}

	got := page.Machines

	for _, gotms := range got {
		for _, sm := range []*fxSchemaMachine{sm0, sm1} {
			if sm.Id != gotms.Id {
				continue
			}
			if !reflect.DeepEqual(gotms, sm) {
				t.Errorf("Unexpected Machines received.")
				t.Logf("Got Machines:")
				t.Logf("%#v", gotms)
				t.Logf("Expected Machines:")
				t.Logf("%#v", sm)
			}
		}
	}

	testMachinesPatchDelete(t, m0, m1)
}

func testMachinesPatchDelete(t *testing.T, m0 platform.Member, m1 platform.Member) {
	sm0 := &fxSchemaMachine{
		Id: m0.ID(),
		Metadata: map[string]string{
			"hostname": fmt.Sprintf("smoke%s", m0.ID()),
		},
		PrimaryIP: m0.IP(),
	}

	sm1 := &fxSchemaMachine{
		Id: m1.ID(),
		Metadata: map[string]string{
			"hostname": fmt.Sprintf("smoke%s", m1.ID()),
		},
		PrimaryIP: m1.IP(),
	}

	msp0 := &machineMetadataOp{
		Operation: "remove",
		Path:      fmt.Sprintf("/%s/metadata/foo", m0.ID()),
		Value:     ValueType{Value: ""},
	}
	msp1 := &machineMetadataOp{
		Operation: "remove",
		Path:      fmt.Sprintf("/%s/metadata/ping", m1.ID()),
		Value:     ValueType{Value: ""},
	}
	msreq, err := json.Marshal([]*machineMetadataOp{msp0, msp1})
	if err != nil {
		t.Fatalf("unexpected error marshalling: %#v", err)
	}
	reqBody := string(msreq)

	respPatch, err := getHTTPResponse("PATCH", m0.Endpoint()+"/fleet/v1/machines", reqBody)
	if err != nil {
		t.Fatal(err)
	}
	defer respPatch.Body.Close()

	if respPatch.StatusCode != http.StatusNoContent {
		t.Fatalf("Expected status %d, got %d.", http.StatusNoContent, respPatch.StatusCode)
	}

	err = checkContentType(respPatch)
	if err != nil {
		t.Fatal(err)
	}

	// Send a normal GET to get list to be compared to an expected list
	respGet, err := getHTTPResponse("GET", m0.Endpoint()+"/fleet/v1/machines", "")
	if err != nil {
		t.Fatal(err)
	}
	defer respGet.Body.Close()

	if respGet.StatusCode != http.StatusOK {
		t.Fatalf("Got HTTP response status %d.", respGet.StatusCode)
	}

	err = checkContentType(respGet)
	if err != nil {
		t.Fatal(err)
	}

	body, rerr := ioutil.ReadAll(respGet.Body)
	if rerr != nil {
		t.Fatalf("Failed to read response body: %v", rerr)
	}

	// unmarshal, compare
	var page fxSchemaMachinePage
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("Received unparsable body: %v", err)
	}

	got := page.Machines

	for _, gotms := range got {
		for _, sm := range []*fxSchemaMachine{sm0, sm1} {
			if sm.Id != gotms.Id {
				continue
			}
			if !reflect.DeepEqual(gotms, sm) {
				t.Errorf("Unexpected Machines received.")
				t.Logf("Got Machines:")
				t.Logf("%#v", gotms)
				t.Logf("Expected Machines:")
				t.Logf("%#v", sm)
			}
		}
	}
}

func TestMachinesPatchBad(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy(t)

	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Test bad operation
	msp0 := &machineMetadataOp{
		Operation: "noop",
		Path:      fmt.Sprintf("/%s/metadata/foo", m0.ID()),
		Value:     ValueType{Value: "bar"},
	}
	msreq, err := json.Marshal([]*machineMetadataOp{msp0})
	if err != nil {
		t.Fatalf("unexpected error marshalling: %#v", err)
	}

	respPatch0, err := getHTTPResponse("PATCH", m0.Endpoint()+"/fleet/v1/machines", string(msreq))
	if err != nil {
		t.Fatal(err)
	}
	defer respPatch0.Body.Close()

	if respPatch0.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d.", http.StatusBadRequest, respPatch0.StatusCode)
	}

	err = checkContentType(respPatch0)
	if err != nil {
		t.Fatal(err)
	}

	// Test bad path
	msp1 := &machineMetadataOp{
		Operation: "add",
		Path:      fmt.Sprintf("/%s/foo", m0.ID()),
		Value:     ValueType{Value: "bar"},
	}
	msreq, err = json.Marshal([]*machineMetadataOp{msp1})
	if err != nil {
		t.Fatalf("unexpected error marshalling: %#v", err)
	}

	respPatch1, err := getHTTPResponse("PATCH", m0.Endpoint()+"/fleet/v1/machines", string(msreq))
	if err != nil {
		t.Fatal(err)
	}
	defer respPatch1.Body.Close()

	if respPatch1.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d.", http.StatusBadRequest, respPatch1.StatusCode)
	}

	err = checkContentType(respPatch1)
	if err != nil {
		t.Fatal(err)
	}

	// Test bad value
	msp2 := &machineMetadataOp{
		Operation: "add",
		Path:      fmt.Sprintf("/%s/foo", m0.ID()),
		Value:     ValueType{Value: ""},
	}
	msreq, err = json.Marshal([]*machineMetadataOp{msp2})
	if err != nil {
		t.Fatalf("unexpected error marshalling: %#v", err)
	}

	respPatch2, err := getHTTPResponse("PATCH", m0.Endpoint()+"/fleet/v1/machines", string(msreq))
	if err != nil {
		t.Fatal(err)
	}
	defer respPatch2.Body.Close()

	if respPatch2.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d.", http.StatusBadRequest, respPatch2.StatusCode)
	}

	err = checkContentType(respPatch2)
	if err != nil {
		t.Fatal(err)
	}
}

func getHTTPResponse(method string, urlPath string, val string) (*http.Response, error) {
	req, err := http.NewRequest(method, urlPath, strings.NewReader(val))
	if err != nil {
		return nil, fmt.Errorf("Failed creating http.Request: %v", err)
	}

	cl := http.Client{}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to run client.Do: %v", err)
	}
	if resp.Body == nil {
		return nil, fmt.Errorf("Got HTTP response nil body")
	}

	return resp, nil
}

func checkContentType(resp *http.Response) error {
	ct := resp.Header.Get("Content-Type")
	if len(ct) == 0 {
		return fmt.Errorf("Response has wrong number of Content-Type values: %v", ct)
	} else if !strings.Contains(ct, "application/json") {
		return fmt.Errorf("Expected application/json, got %v", ct)
	}
	return nil
}
