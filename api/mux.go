package api

import (
	"net/http"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/registry"
)

func NewServeMux(reg registry.Registry) http.Handler {
	sm := http.NewServeMux()

	prefix := "/v1-alpha"
	wireUpMachinesResource(sm, prefix, reg)
	wireUpUnitsResource(sm, prefix, reg)
	sm.HandleFunc(prefix, methodNotAllowedHandler)

	sm.HandleFunc("/", baseHandler)

	lm := &loggingMiddleware{sm}

	return lm
}

type loggingMiddleware struct {
	next http.Handler
}

func (lm *loggingMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.V(1).Infof("HTTP %s %v", req.Method, req.URL)
	lm.next.ServeHTTP(rw, req)
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
