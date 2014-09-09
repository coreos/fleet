package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/pkg"
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
	if su.Name == "" {
		su.Name = item
	}
	if item != su.Name {
		sendError(rw, http.StatusBadRequest, fmt.Errorf("name in URL %q differs from unit name in request body %q", item, su.Name))
		return
	}
	if err := validateName(su.Name); err != nil {
		sendError(rw, http.StatusBadRequest, err)
		return
	}

	eu, err := ur.cAPI.Unit(su.Name)
	if err != nil {
		log.Errorf("Failed fetching Unit(%s) from Registry: %v", su.Name, err)
		sendError(rw, http.StatusInternalServerError, nil)
		return
	}

	if eu == nil {
		if len(su.Options) == 0 {
			err := errors.New("unit does not exist and options field empty")
			sendError(rw, http.StatusConflict, err)
		} else if err := ValidateOptions(su.Options); err != nil {
			sendError(rw, http.StatusBadRequest, err)
		} else {
			ur.create(rw, su.Name, &su)
		}
		return
	}

	if len(su.DesiredState) == 0 {
		err := errors.New("must provide DesiredState to update existing unit")
		sendError(rw, http.StatusConflict, err)
		return
	}

	ur.update(rw, su.Name, su.DesiredState)
}

const (
	// These constants taken from systemd
	unitNameMax    = 256
	digits         = "0123456789"
	lowercase      = "abcdefghijklmnopqrstuvwxyz"
	uppercase      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	alphanumerical = digits + lowercase + uppercase
	validChars     = alphanumerical + `:=+.\@`
)

var validUnitTypes = pkg.NewUnsafeSet(
	"service",
	"socket",
	"busname",
	"target",
	"snapshot",
	"device",
	"mount",
	"automount",
	"swap",
	"timer",
	"path",
	"slice",
	"scope",
)

// validateName ensures that a given unit name is valid; if not, an error is
// returned describing the first issue encountered.
// systemd reference: `unit_name_is_valid` in `unit-name.c`
func validateName(name string) error {
	length := len(name)
	if length == 0 {
		return errors.New("unit name cannot be empty")
	}
	if length > unitNameMax {
		return fmt.Errorf("unit name exceeds maximum length (%d)", unitNameMax)
	}
	dot := strings.Index(name, ".")
	if dot == -1 {
		return errors.New(`unit name must contain "."`)
	}
	if dot == length-1 {
		return errors.New(`unit name cannot end in "."`)
	}
	if suffix := name[dot+1:]; !validUnitTypes.Contains(suffix) {
		return fmt.Errorf("invalid unit type: %q", suffix)
	}
	for _, char := range name[:dot] {
		if !strings.ContainsRune(validChars, char) {
			return fmt.Errorf("invalid character %q in unit name", char)
		}
	}
	if strings.HasPrefix(name, "@") {
		return errors.New(`unit name cannot start in "@"`)
	}
	return nil
}

// ValidateOptions ensures that a set of UnitOptions is valid; if not, an error
// is returned detailing the issue encountered.  If there are several problems
// with a set of options, only the first is returned.
func ValidateOptions(opts []*schema.UnitOption) error {
	uf := schema.MapSchemaUnitOptionsToUnitFile(opts)
	j := &job.Job{
		Unit: *uf,
	}
	conflicts := pkg.NewUnsafeSet(j.Conflicts()...)
	peers := pkg.NewUnsafeSet(j.Peers()...)
	for _, peer := range peers.Values() {
		for _, conflict := range conflicts.Values() {
			matched, _ := path.Match(conflict, peer)
			if matched {
				return fmt.Errorf("unresolvable requirements: peer %q matches conflict %q", peer, conflict)
			}
		}
	}
	hasPeers := peers.Length() != 0
	hasConflicts := conflicts.Length() != 0
	_, hasReqTarget := j.RequiredTarget()
	u := &job.Unit{
		Unit: *uf,
	}
	isGlobal := u.IsGlobal()

	switch {
	case hasReqTarget && hasPeers:
		return errors.New("MachineID cannot be used with Peers")
	case hasReqTarget && hasConflicts:
		return errors.New("MachineID cannot be used with Conflicts")
	case hasReqTarget && isGlobal:
		return errors.New("MachineID cannot be used with Global")
	case isGlobal && hasPeers:
		return errors.New("Global cannot be used with Peers")
	case isGlobal && hasConflicts:
		return errors.New("Global cannot be used with Conflicts")
	}

	return nil
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
