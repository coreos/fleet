package api

import (
	"net/http"
	"path"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
)

func wireUpMachinesResource(mux *http.ServeMux, prefix string, reg registry.Registry) {
	res := path.Join(prefix, "machines")
	mr := machinesResource{reg}
	mux.Handle(res, &mr)
}

type machinesResource struct {
	reg registry.Registry
}

func (mr *machinesResource) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	states, err := mr.reg.GetActiveMachines()
	if err != nil {
		log.Error("Failed fetching MachineStates from Registry: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	page := newMachinePage(states)
	sendResponse(rw, page)
}

func newMachinePage(states []machine.MachineState) *schema.MachinePage {
	smp := schema.MachinePage{Machines: make([]*schema.Machine, 0, len(states))}
	for i, _ := range states {
		ms := states[i]
		sm := mapMachineStateToSchema(&ms)
		smp.Machines = append(smp.Machines, sm)
	}
	return &smp
}

func mapMachineStateToSchema(ms *machine.MachineState) *schema.Machine {
	sm := schema.Machine{
		Id:        ms.ID,
		PrimaryIP: ms.PublicIP,
	}

	sm.Metadata = make(map[string]string, len(ms.Metadata))
	for k, v := range ms.Metadata {
		sm.Metadata[k] = v
	}

	return &sm
}
