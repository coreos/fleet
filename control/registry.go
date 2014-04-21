package control

import (
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
)

// registry-based implementation of the etcd interface

type registryEtcd struct {
	registry *registry.Registry
}

func NewRegistryEtcd(registry *registry.Registry) Etcd {
	return &registryEtcd{registry}
}

func (re *registryEtcd) Hosts() ([]string, error) {
	var hs []string
	ms := re.registry.GetActiveMachines()
	for _, m := range ms {
		hs = append(hs, m.BootID)
	}
	return hs, nil
}

func (re *registryEtcd) Jobs() ([]*JobWithHost, error) {
	var jws []*JobWithHost
	jobs := re.registry.GetAllJobs()
	for _, j := range jobs {
		bootID := re.registry.GetJobTarget(j.Name)
		if bootID != "" {
			jw := &JobWithHost{
				Spec:    JobSpecFrom(&j),
				BootID:  bootID,
				JobName: j.Name,
			}
			jws = append(jws, jw)
		}
	}
	return jws, nil
}

func (re *registryEtcd) Spec(bootID string) (*machine.MachineSpec, error) {
	return re.registry.GetMachineSpec(bootID)
}

func (re *registryEtcd) Specs() (map[string]machine.MachineSpec, error) {
	return re.registry.GetMachineSpecs()
}
