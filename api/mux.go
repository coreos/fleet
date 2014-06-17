package api

import (
	"net/http"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/registry"
)

func NewServeMux(reg registry.Registry) http.Handler {
	prefix := "/v1-alpha"
	sm := http.NewServeMux()
	wireUpMachinesResource(sm, prefix, reg)

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
