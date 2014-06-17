package api

import (
	"encoding/json"
	"net/http"
	"strings"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

func hasValidContentType(req *http.Request) bool {
	for _, hdr := range req.Header["Content-Type"] {
		val := strings.SplitN(hdr, ";", 2)[0]
		val = strings.TrimSpace(val)
		if val != "application/json" {
			return false
		}
	}
	return true
}

// sendResponse attempts to marshal an arbitrary thing to JSON then write
// it to the http.ResponseWriter
func sendResponse(rw http.ResponseWriter, resp interface{}) {
	enc, err := json.Marshal(resp)
	if err != nil {
		log.Error("Failed JSON-encoding HTTP response: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")

	_, err = rw.Write(enc)
	if err != nil {
		log.Error("Failed sending HTTP response body: %v", err)
	}
}
