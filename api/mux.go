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
	"net/http"

	"github.com/coreos/flt/client"
	"github.com/coreos/flt/log"
	"github.com/coreos/flt/registry"
	"github.com/coreos/flt/version"
)

func NewServeMux(reg registry.Registry) http.Handler {
	sm := http.NewServeMux()
	cAPI := &client.RegistryClient{Registry: reg}

	for _, prefix := range []string{"/v1-alpha", "/flt/v1"} {
		wireUpDiscoveryResource(sm, prefix)
		wireUpMachinesResource(sm, prefix, cAPI)
		wireUpStateResource(sm, prefix, cAPI)
		wireUpUnitsResource(sm, prefix, cAPI)
		sm.HandleFunc(prefix, methodNotAllowedHandler)
	}

	sm.HandleFunc("/", baseHandler)

	hdlr := http.Handler(sm)
	hdlr = &loggingMiddleware{hdlr}
	hdlr = &serverInfoMiddleware{hdlr}

	return hdlr
}

type loggingMiddleware struct {
	next http.Handler
}

func (lm *loggingMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.Debugf("HTTP %s %v", req.Method, req.URL)
	lm.next.ServeHTTP(rw, req)
}

type serverInfoMiddleware struct {
	next http.Handler
}

func (si *serverInfoMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Server", "fltd/"+version.Version)
	si.next.ServeHTTP(rw, req)
}

func methodNotAllowedHandler(rw http.ResponseWriter, req *http.Request) {
	sendError(rw, http.StatusMethodNotAllowed, nil)
}

func baseHandler(rw http.ResponseWriter, req *http.Request) {
	var code int
	if req.URL.Path == "/" {
		code = http.StatusMethodNotAllowed
	} else {
		code = http.StatusNotFound
	}

	sendError(rw, code, nil)
}
