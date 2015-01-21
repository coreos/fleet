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
	"errors"
	"net"
	"net/http"

	"github.com/coreos/fleet/log"
)

var unavailable = &unavailableHdlr{}

func NewServer(listeners []net.Listener, hdlr http.Handler) *Server {
	return &Server{
		listeners: listeners,
		api:       hdlr,
		cur:       unavailable,
	}
}

type Server struct {
	listeners []net.Listener
	api       http.Handler
	cur       http.Handler
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.cur.ServeHTTP(rw, req)
}

func (s *Server) Serve() {
	for i, _ := range s.listeners {
		l := s.listeners[i]
		go func() {
			err := http.Serve(l, s)
			if err != nil {
				log.Errorf("Failed serving HTTP on listener: %v", l.Addr())
			}
		}()
	}
}

// Available switches the Server's HTTP handler from a generic 503 Unavailable
// response to the actual API. Once the provided channel is closed, the API is
// torn back down and 503 responses are served.
func (s *Server) Available(stop <-chan struct{}) {
	s.cur = s.api
	<-stop
	s.cur = unavailable
}

type unavailableHdlr struct{}

func (uh *unavailableHdlr) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	sendError(rw, http.StatusServiceUnavailable, errors.New("fleet server unable to communicate with etcd"))
}
