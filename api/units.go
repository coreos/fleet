package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
	"github.com/coreos/fleet/unit"
)

func wireUpUnitsResource(mux *http.ServeMux, prefix string, reg registry.Registry) {
	res := path.Join(prefix, "units")
	ur := unitsResource{reg}
	mux.Handle(res, &ur)
}

type unitsResource struct {
	reg registry.Registry
}

func (ur *unitsResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		ur.list(rw, req)
	case "DELETE":
		ur.destroy(rw, req)
	default:
		rw.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (ur *unitsResource) destroy(rw http.ResponseWriter, req *http.Request) {
	if validateContentType(req) != nil {
		rw.WriteHeader(http.StatusNotAcceptable)
		return
	}

	var c schema.DeletableUnitCollection
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&c)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, unit := range c.Units {
		err := ur.reg.DestroyJob(unit.Name)
		if err != nil {
			//TODO(bcwaldon): Which error is correct here?
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	rw.WriteHeader(http.StatusNoContent)
}

func (ur *unitsResource) list(rw http.ResponseWriter, req *http.Request) {
	token, err := findNextPageToken(req.URL)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	if token == nil {
		def := DefaultPageToken()
		token = &def
	}

	page, err := getUnitPage(ur.reg, *token)
	if err != nil {
		log.Errorf("Failed fetching page of Units: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	sendResponse(rw, page)
}

func getUnitPage(reg registry.Registry, tok PageToken) (*schema.UnitPage, error) {
	all, err := reg.GetAllJobs()
	if err != nil {
		return nil, err
	}

	page := extractUnitPage(all, tok)
	return page, nil
}

func extractUnitPage(all []job.Job, tok PageToken) *schema.UnitPage {
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

	return newUnitPage(items, next)
}

func newUnitPage(items []job.Job, tok *PageToken) *schema.UnitPage {
	sup := schema.UnitPage{
		Units: make([]*schema.Unit, 0, len(items)),
	}

	if tok != nil {
		sup.NextPageToken = tok.Encode()
	}

	for _, j := range items {
		sup.Units = append(sup.Units, mapJobToSchema(&j))
	}
	return &sup
}

func mapJobToSchema(j *job.Job) *schema.Unit {
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

	return &su
}

func encodeUnitContents(u *unit.Unit) string {
	return base64.StdEncoding.EncodeToString([]byte(u.Raw))
}
