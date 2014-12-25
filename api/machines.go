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
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"regexp"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/schema"
)

var (
	metadataPathRegex = regexp.MustCompile("^/([^/]+)/metadata/([A-Za-z0-9_.-]+$)")
)

func wireUpMachinesResource(mux *http.ServeMux, prefix string, cAPI client.API) {
	res := path.Join(prefix, "machines")
	mr := machinesResource{cAPI}
	mux.Handle(res, &mr)
}

type machinesResource struct {
	cAPI client.API
}

type machineMetadataOp struct {
	Operation string `json:"op"`
	Path      string
	Value     string
}

func (mr *machinesResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		mr.list(rw, req)
	case "PATCH":
		mr.patch(rw, req)
	default:
		sendError(rw, http.StatusMethodNotAllowed, errors.New("only GET and PATCH supported against this resource"))
	}
}

func (mr *machinesResource) list(rw http.ResponseWriter, req *http.Request) {
	token, err := findNextPageToken(req.URL)
	if err != nil {
		sendError(rw, http.StatusBadRequest, err)
		return
	}

	if token == nil {
		def := DefaultPageToken()
		token = &def
	}

	page, err := getMachinePage(mr.cAPI, *token)
	if err != nil {
		log.Errorf("Failed fetching page of Machines: %v", err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	sendResponse(rw, http.StatusOK, page)
}

func (mr *machinesResource) patch(rw http.ResponseWriter, req *http.Request) {
	var ops []machineMetadataOp
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&ops); err != nil {
		sendError(rw, http.StatusBadRequest, err)
		return
	}

	for _, op := range ops {
		if op.Operation != "add" && op.Operation != "remove" && op.Operation != "replace" {
			sendError(rw, http.StatusBadRequest, errors.New("invalid op: expect add, remove, or replace"))
			return
		}

		if metadataPathRegex.FindStringSubmatch(op.Path) == nil {
			sendError(rw, http.StatusBadRequest, errors.New("machine metadata path invalid"))
			return
		}

		if op.Operation != "remove" && len(op.Value) == 0 {
			sendError(rw, http.StatusBadRequest, errors.New("invalid value: add and replace require a value"))
			return
		}
	}

	for _, op := range ops {
		// regex already validated above
		s := metadataPathRegex.FindStringSubmatch(op.Path)
		machID := s[1]
		key := s[2]

		if op.Operation == "remove" {
			err := mr.cAPI.DeleteMachineMetadata(machID, key)
			if err != nil {
				sendError(rw, http.StatusInternalServerError, err)
				return
			}
		} else {
			err := mr.cAPI.SetMachineMetadata(machID, key, op.Value)
			if err != nil {
				sendError(rw, http.StatusInternalServerError, err)
				return
			}
		}
	}
	sendResponse(rw, http.StatusNoContent, "")
}

func getMachinePage(cAPI client.API, tok PageToken) (*schema.MachinePage, error) {
	all, err := cAPI.Machines()
	if err != nil {
		return nil, err
	}

	page := extractMachinePage(all, tok)
	return page, nil
}

func extractMachinePage(all []machine.MachineState, tok PageToken) *schema.MachinePage {
	total := len(all)

	startIndex := int((tok.Page - 1) * tok.Limit)
	stopIndex := int(tok.Page * tok.Limit)

	var items []machine.MachineState
	var next *PageToken

	if startIndex < total {
		if stopIndex > total {
			stopIndex = total
		} else {
			n := tok.Next()
			next = &n
		}

		items = all[startIndex:stopIndex]
	}

	return newMachinePage(items, next)
}

func newMachinePage(items []machine.MachineState, tok *PageToken) *schema.MachinePage {
	smp := schema.MachinePage{
		Machines: make([]*schema.Machine, 0, len(items)),
	}

	if tok != nil {
		smp.NextPageToken = tok.Encode()
	}

	for _, sm := range items {
		smp.Machines = append(smp.Machines, schema.MapMachineStateToSchema(&sm))
	}
	return &smp
}
