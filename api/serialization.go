package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/fleet/third_party/code.google.com/p/google-api-go-client/googleapi"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

func validateContentType(req *http.Request) error {
	values := req.Header["Content-Type"]
	count := len(values)

	if count != 1 {
		return fmt.Errorf("expected 1 Content-Type, got %d", count)
	}

	val := strings.SplitN(values[0], ";", 2)[0]
	val = strings.TrimSpace(val)
	if val != "application/json" {
		return errors.New("only acceptable Content-Type is application/json")
	}

	return nil
}

// sendResponse attempts to marshal an arbitrary thing to JSON then write
// it to the http.ResponseWriter
func sendResponse(rw http.ResponseWriter, code int, resp interface{}) {
	enc, err := json.Marshal(resp)
	if err != nil {
		log.Errorf("Failed JSON-encoding HTTP response: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(code)

	_, err = rw.Write(enc)
	if err != nil {
		log.Errorf("Failed sending HTTP response body: %v", err)
	}
}

type errorResponse struct {
	Error *googleapi.Error `json:"error"`
}

// sendError builds a googleapi.Error entity from the given arguments, serializing
// the object into the provided http.ResponseWriter
func sendError(rw http.ResponseWriter, code int, err error) {
	resp := errorResponse{Error: &googleapi.Error{Code: code}}
	if err != nil {
		resp.Error.Message = err.Error()
	}
	sendResponse(rw, code, resp)
}
