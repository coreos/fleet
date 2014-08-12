package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func wireUpUnitsResource(mux *http.ServeMux, prefix string, reg registry.Registry) {
	base := path.Join(prefix, "units")
	ur := unitsResource{reg, base}
	mux.Handle(base, &ur)
	mux.Handle(base+"/", &ur)
}

type unitsResource struct {
	reg      registry.Registry
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

	var dus schema.DesiredUnitState
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&dus)
	if err != nil {
		sendError(rw, http.StatusBadRequest, fmt.Errorf("unable to decode body: %v", err))
		return
	}

	u, err := ur.reg.Unit(item)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	var uf *unit.UnitFile
	if len(dus.Options) > 0 {
		uf = schema.MapSchemaToUnitFile(dus.Options)
	}

	// TODO(bcwaldon): Assert value of DesiredState is launched, loaded or inactive
	ds := job.JobState(dus.DesiredState)

	if u != nil {
		ur.update(rw, u, ds)
	} else if uf != nil {
		ur.create(rw, item, ds, uf)
	} else {
		sendError(rw, http.StatusConflict, errors.New("unit does not exist and no fileContents provided"))
	}
}

func (ur *unitsResource) create(rw http.ResponseWriter, item string, ds job.JobState, uf *unit.UnitFile) {
	u := job.Unit{Name: item, Unit: *uf}
	if err := ur.reg.CreateUnit(&u); err != nil {
		log.Errorf("Failed creating Unit(%s) in Registry: %v", u.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if err := ur.reg.SetUnitTargetState(u.Name, ds); err != nil {
		log.Errorf("Failed setting target state of Unit(%s): %v", u.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) update(rw http.ResponseWriter, u *job.Unit, ds job.JobState) {
	err := ur.reg.SetUnitTargetState(u.Name, ds)
	if err != nil {
		log.Errorf("Failed setting target state of Unit(%s): %v", u.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) destroy(rw http.ResponseWriter, req *http.Request, item string) {
	u, err := ur.reg.Unit(item)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if u == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit does not exist"))
		return
	}

	err = ur.reg.DestroyUnit(item)
	if err != nil {
		log.Errorf("Failed destroying Job(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) get(rw http.ResponseWriter, req *http.Request, item string) {
	u, err := ur.reg.Unit(item)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if u == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit does not exist"))
		return
	}

	j := job.Job{
		Name:        u.Name,
		Unit:        u.Unit,
		TargetState: u.TargetState,
	}

	su, err := ur.reg.ScheduledUnit(item)
	if err != nil {
		log.Errorf("Failed fetching ScheduledUnit(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if su != nil {
		j.State = su.State
		j.TargetMachineID = su.TargetMachineID
	}

	s, err := schema.MapJobToSchema(&j)
	if err != nil {
		log.Errorf("Failed mapping Job(%s) to schema: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	sendResponse(rw, http.StatusOK, *s)
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

	page, err := getUnitPage(ur.reg, *token)
	if err != nil {
		log.Errorf("Failed fetching page of Units: %v", err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	sendResponse(rw, http.StatusOK, page)
}

func getUnitPage(reg registry.Registry, tok PageToken) (*schema.UnitPage, error) {
	units, err := reg.Units()
	if err != nil {
		return nil, err
	}

	sUnits, err := reg.Schedule()
	if err != nil {
		return nil, err
	}

	sUnitMap := make(map[string]*job.ScheduledUnit)
	for _, sUnit := range sUnits {
		sUnit := sUnit
		sUnitMap[sUnit.Name] = &sUnit
	}

	jobs := make([]job.Job, len(units))
	for i, u := range units {
		j := job.Job{
			Name:        u.Name,
			Unit:        u.Unit,
			TargetState: u.TargetState,
		}

		if sUnit, ok := sUnitMap[u.Name]; ok {
			j.TargetMachineID = sUnit.TargetMachineID
			j.State = sUnit.State
		}

		jobs[i] = j
	}

	page, err := extractUnitPage(reg, jobs, tok)
	if err != nil {
		return nil, err
	}

	return page, nil
}

func extractUnitPage(reg registry.Registry, all []job.Job, tok PageToken) (*schema.UnitPage, error) {
	total := len(all)

	startIndex := int((tok.Page - 1) * tok.Limit)
	stopIndex := int(tok.Page * tok.Limit)

	var items []job.Job
	var next *PageToken

	if startIndex < total {
		if stopIndex > total {
			stopIndex = total
		} else {
			n := tok.Next()
			next = &n
		}

		items = all[startIndex:stopIndex]
	}

	return newUnitPage(reg, items, next)
}

func newUnitPage(reg registry.Registry, items []job.Job, tok *PageToken) (*schema.UnitPage, error) {
	sup := schema.UnitPage{
		Units: make([]*schema.Unit, 0, len(items)),
	}

	if tok != nil {
		sup.NextPageToken = tok.Encode()
	}

	for _, j := range items {
		u, err := schema.MapJobToSchema(&j)
		if err != nil {
			return nil, err
		}
		sup.Units = append(sup.Units, u)
	}
	return &sup, nil
}
