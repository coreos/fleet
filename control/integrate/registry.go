package integrate

import (
	"github.com/coreos/fleet/control"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

// registry-based implementation of the ClusterCentral interface

type registryClusterCentral struct {
	registry *registry.Registry
}

func NewRegistryClusterCentral(registry *registry.Registry) control.ClusterCentral {
	return &registryClusterCentral{registry}
}

func (re *registryClusterCentral) Hosts() ([]string, error) {
	var hs []string
	ms := re.registry.GetActiveMachines()
	for _, m := range ms {
		hs = append(hs, m.BootID)
	}
	return hs, nil
}

func (re *registryClusterCentral) Jobs() ([]*control.JobWithHost, error) {
	var jws []*control.JobWithHost
	jobs := re.registry.GetAllJobs()
	for _, j := range jobs {
		bootID := re.registry.GetJobTarget(j.Name)
		if bootID != "" {
			jw := &control.JobWithHost{
				Spec:    JobSpecFrom(&j),
				BootID:  bootID,
				JobName: j.Name,
			}
			jws = append(jws, jw)
		}
	}
	return jws, nil
}

func (re *registryClusterCentral) Spec(bootID string) (*machine.MachineSpec, error) {
	return re.registry.GetMachineSpec(bootID)
}

func (re *registryClusterCentral) Specs() (map[string]machine.MachineSpec, error) {
	return re.registry.GetMachineSpecs()
}
