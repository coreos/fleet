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
	"errors"
	"net/http"
	"path"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/schema"
)

func wireUpStateResource(mux *http.ServeMux, prefix string, tokenLimit int, cAPI client.API) {
	base := path.Join(prefix, "state")
	sr := stateResource{cAPI, base, uint16(tokenLimit)}
	mux.Handle(base, &sr)
	mux.Handle(base+"/", &sr)
}

type stateResource struct {
	cAPI       client.API
	basePath   string
	tokenLimit uint16
}

func (sr *stateResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if isCollectionPath(sr.basePath, req.URL.Path) {
		switch req.Method {
		case "GET":
			sr.list(rw, req)
		default:
			sendError(rw, http.StatusMethodNotAllowed, errors.New("only GET supported against this resource"))
		}
	} else if item, ok := isItemPath(sr.basePath, req.URL.Path); ok {
		switch req.Method {
		case "GET":
			sr.get(rw, req, item)
		default:
			sendError(rw, http.StatusMethodNotAllowed, errors.New("only GET supported against this resource"))
		}
	} else {
		sendError(rw, http.StatusNotFound, nil)
	}
}

func (sr *stateResource) list(rw http.ResponseWriter, req *http.Request) {
	token, err := findNextPageToken(req.URL, sr.tokenLimit)
	if err != nil {
		sendError(rw, http.StatusBadRequest, err)
		return
	}

	if token == nil {
		def := DefaultPageToken(sr.tokenLimit)
		token = &def
	}

	var machineID, unitName string
	for _, val := range req.URL.Query()["machineID"] {
		machineID = val
		break
	}
	for _, val := range req.URL.Query()["unitName"] {
		unitName = val
		break
	}

	page, err := getUnitStatePage(sr.cAPI, machineID, unitName, *token)
	if err != nil {
		log.Errorf("Failed fetching page of UnitStates: %v", err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	sendResponse(rw, http.StatusOK, &page)
}

func (sr *stateResource) get(rw http.ResponseWriter, req *http.Request, item string) {
	us, err := sr.cAPI.UnitState(item)
	if err != nil {
		log.Errorf("Failed fetching UnitState(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if us == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit state does not exist"))
		return
	}

	sendResponse(rw, http.StatusOK, *us)
}

func getUnitStatePage(cAPI client.API, machineID, unitName string, tok PageToken) (*schema.UnitStatePage, error) {
	states, err := cAPI.UnitStates()
	if err != nil {
		return nil, err
	}
	var filtered []*schema.UnitState
	for _, us := range states {
		if machineID != "" && machineID != us.MachineID {
			continue
		}
		if unitName != "" && unitName != us.Name {
			continue
		}
		filtered = append(filtered, us)
	}

	items, next := extractUnitStatePageData(filtered, tok)
	page := schema.UnitStatePage{
		States: items,
	}

	if next != nil {
		page.NextPageToken = next.Encode()
	}

	return &page, nil
}

func extractUnitStatePageData(all []*schema.UnitState, tok PageToken) (items []*schema.UnitState, next *PageToken) {
	total := len(all)

	startIndex := int((tok.Page - 1) * tok.Limit)
	stopIndex := int(tok.Page * tok.Limit)

	if startIndex < total {
		if stopIndex > total {
			stopIndex = total
		} else {
			n := tok.Next()
			next = &n
		}

		items = all[startIndex:stopIndex]
	}

	return
}
