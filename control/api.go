package control

import (
	"errors"

	"github.com/coreos/fleet/machine"
)

var (
	ErrClusterFull                  = errors.New("insufficient resources available to schedule job")
	ErrRequiredHostUnavailable      = errors.New("required host is not available")
	ErrDependOnHostUnavailable      = errors.New("host with required dependencies is not available")
	ErrConflictsWithHostUnavailable = errors.New("host that doesn't conflict is not available")
	ErrAllAgentsFailedToRun         = errors.New("no agent was able to run job")
)

// JobSpec defines the requirements of a job in the cluster.
type JobSpec struct {
	// jobs are identified by name, unique within the cluster
	Name string
	// requires to run on a specific machine
	RequiresHost string
	// slice of job names that already need to run on the same machine
	// dependency graph needs to be acyclic (otherwise it's unschedulable)
	DependsOn []string
	// slice of job name glob patterns that are not allowed to run on the same machine
	ConflictsWith []string
	// how much memory job requires, in MB
	MemoryRequired int
	// how many cores job requires: 100=1core, 50=0.5core, 200=2cores, etc
	CoresRequired int
	// how much local disk space job requires, in MB
	DiskSpaceRequired int
}

// JobControl schedules jobs in the cluster.
type JobControl interface {
	// ScheduleJob returns a slice of boot ids of hosts that can run the specified job.
	// Slice is sorted by what job control considers best suited: first boot id is
	// best suited to run the job, followed by the second boot id ...etc.
	// Returns ErrClusterFull if cluster
	// cannot fit the job. Returns one of the other errors defined
	// above if clauses of the job couldn't be satisfied.
	// Returns network errors if communication with Etcd failed.
	ScheduleJob(spec *JobSpec) ([]string, error)

	// a job control needs to listen to these four events in the cluster
	// to function properly. somebody needs to watch etcd and feed them into
	// this job control
	JobScheduled(jobName string, bootID string, spec *JobSpec)
	JobDowned(jobName string, bootID string, spec *JobSpec)
	HostDown(bootID string)
	HostUp(bootID string)
}

// JobWithHost is job with the host on which it runs.
type JobWithHost struct {
	Spec    *JobSpec
	BootID  string
	JobName string
}

// Etcd interface specifies what job control will ask etcd.
type Etcd interface {
	// Give me all the currently active hosts
	// (hosts that have an agent running, maintaining heartbeat with etcd)
	Hosts() ([]string, error)
	// Give me all the jobs running in the cluster right now
	Jobs() ([]*JobWithHost, error)
	// Spec returns the machine spec of the given host.
	Spec(bootID string) (*machine.MachineSpec, error)
	// All specs
	Specs() (map[string]machine.MachineSpec, error)
}
