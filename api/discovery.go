package api

import (
	"fmt"
	"net/http"
	"path"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/schema"
)

func wireUpDiscoveryResource(mux *http.ServeMux, prefix string) {
	base := path.Join(prefix, "discovery.json")
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
