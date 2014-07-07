package resource

// ResourceTuple groups together CPU, memory and disk space. This could be
// total, available or consumed. It could also be used by job resource requirements.
type ResourceTuple struct {
	// in hundreds, ie 100=1core, 50=0.5core, 200=2cores, etc
	Cores int
	// in MB
	Memory int
	// in MB
	Disk int
}

const (
	// TODO(jonboulle): make these configurable
	HostCores  = 100
	HostMemory = 256
	HostDisk   = 0
)

// HostResources represents a set of resources that fleet considers reserved
// for the host, i.e. outside of any units it is running
var HostResources = ResourceTuple{
	HostCores,
	HostMemory,
	HostDisk,
}

// Sum aggregates a number of ResourceTuples into a single entity
func Sum(resources ...ResourceTuple) (res ResourceTuple) {
	for _, r := range resources {
		res.Cores += r.Cores
		res.Memory += r.Memory
		res.Disk += r.Disk
	}
	return
}

// Sub returns a ResourceTuple representing the difference between two
// ResourceTuples
func Sub(r1, r2 ResourceTuple) (res ResourceTuple) {
	res.Cores = r1.Cores - r2.Cores
	res.Memory = r1.Memory - r2.Memory
	res.Disk = r1.Disk - r2.Disk
	return
}
