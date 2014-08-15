package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/schema"
)

func wireUpUnitsResource(mux *http.ServeMux, prefix string, cAPI client.API) {
	base := path.Join(prefix, "units")
	ur := unitsResource{cAPI, base}
	mux.Handle(base, &ur)
	mux.Handle(base+"/", &ur)
}

type unitsResource struct {
	cAPI     client.API
	basePath string
}

func (ur *unitsResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if isCollectionPath(ur.basePath, req.URL.Path) {
		switch req.Method {
		case "GET":
			ur.list(rw, req)
		default:
			sendError(rw, http.StatusMethodNotAllowed, errors.New("only GET supported against this resource"))
		}
	} else if item, ok := isItemPath(ur.basePath, req.URL.Path); ok {
		switch req.Method {
		case "GET":
			ur.get(rw, req, item)
		case "DELETE":
			ur.destroy(rw, req, item)
		case "PUT":
			ur.set(rw, req, item)
		default:
			sendError(rw, http.StatusMethodNotAllowed, errors.New("only GET, PUT and DELETE supported against this resource"))
		}
	} else {
		sendError(rw, http.StatusNotFound, nil)
	}
}

func (ur *unitsResource) set(rw http.ResponseWriter, req *http.Request, item string) {
	if validateContentType(req) != nil {
		sendError(rw, http.StatusNotAcceptable, errors.New("application/json is only supported Content-Type"))
		return
	}

	var su schema.Unit
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&su)
	if err != nil {
		sendError(rw, http.StatusBadRequest, fmt.Errorf("unable to decode body: %v", err))
		return
	}

	eu, err := ur.cAPI.Unit(item)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if eu == nil && len(su.Options) == 0 {
		err := errors.New("unit does not exist and options field empty")
		sendError(rw, http.StatusConflict, err)
		return
	}

	if eu == nil {
		ur.create(rw, item, &su)
		return
	}

	if len(su.DesiredState) == 0 {
		err := errors.New("must provide DesiredState to update existing unit")
		sendError(rw, http.StatusConflict, err)
		return
	}

	ur.update(rw, item, su.DesiredState)
}

func (ur *unitsResource) create(rw http.ResponseWriter, name string, u *schema.Unit) {
	if err := ur.cAPI.CreateUnit(u); err != nil {
		log.Errorf("Failed creating Unit(%s) in Registry: %v", u.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) update(rw http.ResponseWriter, item, ds string) {
	if err := ur.cAPI.SetUnitTargetState(item, ds); err != nil {
		log.Errorf("Failed setting target state of Unit(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) destroy(rw http.ResponseWriter, req *http.Request, item string) {
	u, err := ur.cAPI.Unit(item)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if u == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit does not exist"))
		return
	}

	err = ur.cAPI.DestroyUnit(item)
	if err != nil {
		log.Errorf("Failed destroying Unit(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) get(rw http.ResponseWriter, req *http.Request, item string) {
	u, err := ur.cAPI.Unit(item)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if u == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit does not exist"))
		return
	}

	sendResponse(rw, http.StatusOK, *u)
}

func (ur *unitsResource) list(rw http.ResponseWriter, req *http.Request) {
	token, err := findNextPageToken(req.URL)
	if err != nil {
		sendError(rw, http.StatusBadRequest, err)
		return
	}

	if token == nil {
		def := DefaultPageToken()
		token = &def
	}

	page, err := getUnitPage(ur.cAPI, *token)
	if err != nil {
		log.Errorf("Failed fetching page of Units: %v", err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	sendResponse(rw, http.StatusOK, page)
}

func getUnitPage(cAPI client.API, tok PageToken) (*schema.UnitPage, error) {
	units, err := cAPI.Units()
	if err != nil {
		return nil, err
	}

	items, next := extractUnitPageData(units, tok)
	page := schema.UnitPage{
		Units: items,
	}

	if next != nil {
		page.NextPageToken = next.Encode()
	}

	return &page, nil
}

func extractUnitPageData(all []*schema.Unit, tok PageToken) (items []*schema.Unit, next *PageToken) {
	total := len(all)

	startIndex := int((tok.Page - 1) * tok.Limit)
	stopIndex := int(tok.Page * tok.Limit)

	if startIndex < total {
		if stopIndex > total {
			stopIndex = total
		} else {
			n := tok.Next()
			next = &n
		}

		items = all[startIndex:stopIndex]
	}

	return
}
