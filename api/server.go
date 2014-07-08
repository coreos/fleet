package api

import (
	"errors"
	"net"
	"net/http"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"
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
				log.Errorf("Failed serving HTTP on listener: %s", l.Addr)
			}
		}()
	}
}

// Available switches the Server's HTTP handler from a generic 503 Unavailable
// response to the actual API. Once the provided channel is closed, the API is
// torn back down and 503 responses are served.
func (s *Server) Available(stop chan bool) {
	s.cur = s.api
	<-stop
	s.cur = unavailable
}

type unavailableHdlr struct{}

func (uh *unavailableHdlr) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	sendError(rw, http.StatusServiceUnavailable, errors.New("fleet server currently unavailable"))
}
