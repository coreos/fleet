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
		case "DELETE":
			ur.destroy(rw, req)
		case "POST":
			ur.set(rw, req)
		default:
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	} else if item, ok := isItemPath(ur.basePath, req.URL.Path); ok {
		switch req.Method {
		case "GET":
			ur.get(rw, req, item)
		default:
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	} else {
		rw.WriteHeader(http.StatusNotFound)
	}
}

func (ur *unitsResource) set(rw http.ResponseWriter, req *http.Request) {
	if validateContentType(req) != nil {
		rw.WriteHeader(http.StatusNotAcceptable)
		return
	}

	var c schema.DesiredUnitStateCollection
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&c)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	for _, c := range c.Units {
		j, err := ur.reg.Job(c.Name)
		if err != nil {
			log.Errorf("Failed fetching Job(%s) from Registry: %v", c.Name, err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if j == nil {
			u, err := decodeUnitContents(c.FileContents)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			j = job.NewJob(c.Name, *u)

			if err = ur.reg.CreateJob(j); err != nil {
				log.Errorf("Failed creating Job(%s) in Registry: %v", j.Name, err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		if err := ur.reg.SetJobTargetState(j.Name, job.JobState(c.DesiredState)); err != nil {
			log.Errorf("Failed setting target state of Job(%s): %v", j.Name, err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	rw.WriteHeader(http.StatusNoContent)
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

func (ur *unitsResource) get(rw http.ResponseWriter, req *http.Request, item string) {
	j, err := ur.reg.Job(item)
	if err != nil {
		log.Errorf("Failed fetching Unit: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if j == nil {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	tgt, err := ur.reg.GetJobTarget(j.Name)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	u := mapJobToSchema(j)
	u.TargetMachineID = tgt

	sendResponse(rw, *u)
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
	all, err := reg.Jobs()
	if err != nil {
		return nil, err
	}

	page := extractUnitPage(all, tok)

	err = setUnitPageTargets(reg, page)
	if err != nil {
		return nil, err
	}

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

func setUnitPageTargets(reg registry.Registry, page *schema.UnitPage) error {
	for i, _ := range page.Units {
		tgt, err := reg.GetJobTarget(page.Units[i].Name)
		if err != nil {
			return err
		}
		page.Units[i].TargetMachineID = tgt
	}
	return nil
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
