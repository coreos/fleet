package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"

	log "github.com/golang/glog"

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

	j, err := ur.reg.Job(item)
	if err != nil {
		log.Errorf("Failed fetching Job(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	var u *unit.Unit
	if len(dus.FileContents) > 0 {
		u, err = decodeUnitContents(dus.FileContents)
		if err != nil {
			sendError(rw, http.StatusBadRequest, fmt.Errorf("invalid fileContents: %v", err))
			return
		}
	}

	// TODO(bcwaldon): Assert value of DesiredState is launched, loaded or inactive
	ds := job.JobState(dus.DesiredState)

	if j != nil {
		ur.update(rw, j, ds, u)
	} else if u != nil {
		ur.create(rw, item, ds, u)
	} else {
		sendError(rw, http.StatusConflict, errors.New("unit does not exist and no fileContents provided"))
	}
}

func (ur *unitsResource) create(rw http.ResponseWriter, item string, ds job.JobState, u *unit.Unit) {
	j := job.NewJob(item, *u)

	if err := ur.reg.CreateJob(j); err != nil {
		log.Errorf("Failed creating Job(%s) in Registry: %v", j.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if err := ur.reg.SetJobTargetState(j.Name, ds); err != nil {
		log.Errorf("Failed setting target state of Job(%s): %v", j.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) update(rw http.ResponseWriter, j *job.Job, ds job.JobState, cmp *unit.Unit) {
	// Assert that the Job's Unit matches the Unit in the request, if provided
	if cmp != nil && cmp.Hash() != j.Unit.Hash() {
		sendError(rw, http.StatusConflict, errors.New("hash of provided fileContents does not match that of existing unit"))
		return
	}

	err := ur.reg.SetJobTargetState(j.Name, ds)
	if err != nil {
		log.Errorf("Failed setting target state of Job(%s): %v", j.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) destroy(rw http.ResponseWriter, req *http.Request, item string) {
	if validateContentType(req) != nil {
		sendError(rw, http.StatusNotAcceptable, errors.New("application/json is only supported Content-Type"))
		return
	}

	var du schema.DeletableUnit
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&du)
	if err != nil {
		sendError(rw, http.StatusBadRequest, fmt.Errorf("unable to decode body: %v", err))
		return
	}

	var u *unit.Unit
	if len(du.FileContents) > 0 {
		u, err = decodeUnitContents(du.FileContents)
		if err != nil {
			sendError(rw, http.StatusBadRequest, fmt.Errorf("invalid fileContents: %v", err))
			return
		}
	}

	j, err := ur.reg.Job(item)
	if err != nil {
		log.Errorf("Failed fetching Job(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if j == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit does not exist"))
		return
	}

	if u != nil && u.Hash() != j.Unit.Hash() {
		sendError(rw, http.StatusConflict, errors.New("hash of provided fileContents does not match that of existing unit"))
		return
	}

	err = ur.reg.DestroyJob(item)
	if err != nil {
		log.Errorf("Failed destroying Job(%s): %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) get(rw http.ResponseWriter, req *http.Request, item string) {
	j, err := ur.reg.Job(item)
	if err != nil {
		log.Errorf("Failed fetching Job(%s) from Registry: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if j == nil {
		sendError(rw, http.StatusNotFound, errors.New("unit does not exist"))
		return
	}

	u, err := mapJobToSchema(ur.reg, j)
	if err != nil {
		log.Errorf("Failed mapping Job(%s) to schema: %v", item, err)
		sendError(rw, http.StatusInternalServerError, nil)
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

	page, err := getUnitPage(ur.reg, *token)
	if err != nil {
		log.Errorf("Failed fetching page of Units: %v", err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	sendResponse(rw, http.StatusOK, page)
}

func getUnitPage(reg registry.Registry, tok PageToken) (*schema.UnitPage, error) {
	all, err := reg.Jobs()
	if err != nil {
		return nil, err
	}

	page, err := extractUnitPage(reg, all, tok)
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
		u, err := mapJobToSchema(reg, &j)
		if err != nil {
			return nil, err
		}
		sup.Units = append(sup.Units, u)
	}
	return &sup, nil
}

func mapJobToSchema(reg registry.Registry, j *job.Job) (*schema.Unit, error) {
	su := schema.Unit{
		Name:         j.Name,
		FileHash:     j.UnitHash.String(),
		FileContents: encodeUnitContents(&j.Unit),
	}

	if j.State != nil {
		su.CurrentState = string(*(j.State))
	}

	if j.UnitState != nil {
		su.Systemd = &schema.SystemdState{
			LoadState:   j.UnitState.LoadState,
			ActiveState: j.UnitState.ActiveState,
			SubState:    j.UnitState.SubState,
		}
		if j.UnitState.MachineState != nil {
			su.Systemd.MachineID = j.UnitState.MachineState.ID
		}
	}

	tgtMachID, err := reg.JobTarget(j.Name)
	if err != nil {
		return nil, err
	}

	su.TargetMachineID = tgtMachID

	tgtState, err := reg.JobTargetState(j.Name)
	if err != nil {
		return nil, err
	}

	if tgtState != nil {
		su.DesiredState = string(*tgtState)
	}

	return &su, nil
}

func encodeUnitContents(u *unit.Unit) string {
	return base64.StdEncoding.EncodeToString([]byte(u.Raw))
}

func decodeUnitContents(c string) (*unit.Unit, error) {
	dec, err := base64.StdEncoding.DecodeString(c)
	if err != nil {
		return nil, err
	}

	return unit.NewUnit(string(dec))
}
