package api

import (
	"net/http"
	"path"

	"github.com/coreos/fleet/registry"
)

func wireUpHealthResource(mux *http.ServeMux, prefix string, reg registry.Registry) {
	res := path.Join(prefix, "health")
	hr := healthResource{reg}
	mux.Handle(res, &hr)
}

type healthResource struct {
	reg registry.Registry
}

func (hr *healthResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	hr.check(rw, req)
}

func (hr *healthResource) check(rw http.ResponseWriter, req *http.Request) {
	rw.Write([]byte("OK"))
}
