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
	"fmt"
	"net/http"
	"path"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/schema"
)

func wireUpDiscoveryResource(mux *http.ServeMux, prefix string) {
	base := path.Join(prefix, "discovery")
	dr := discoveryResource{}
	mux.Handle(base, &dr)
}

type discoveryResource struct{}

func (dr *discoveryResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		sendError(rw, http.StatusBadRequest, fmt.Errorf("only HTTP GET supported against this resource"))
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(200)
	if _, err := rw.Write([]byte(schema.DiscoveryJSON)); err != nil {
		log.Errorf("Failed sending HTTP response body: %v", err)
	}
}
