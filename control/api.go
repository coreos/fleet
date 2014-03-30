package control

import "errors"

var (
	ErrClusterFull                  = errors.New("insufficient resources available to schedule job")
	ErrRequiredHostUnavailable      = errors.New("required host is not available")
	ErrDependOnHostUnavailable      = errors.New("host with required dependencies is not available")
	ErrConflictsWithHostUnavailable = errors.New("host that doesn't conflict is not available")
	ErrAllAgentsFailedToRun         = errors.New("no agent was able to run job")
)

type MachineSpec struct {
	// in hundreds, ie 100=1core, 50=0.5core, 200=2cores, etc
	Cores int
	// in MB
	Memory int
	// in GB
	LocalDiskSpace int
}

type JobSpec struct {
	// jobs are identified by name
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
	// how much local disk space job requires, in GB
	LocalDiskSpaceRequired int
	// system.d unit file description of job
	Unit string
}

type JobControl interface {
	// returns a unique job id for the scheduled job
	ScheduleJob(spec *JobSpec) (string, error)

	// a job control needs to listen to these three events in the cluster
	// to function properly. somebody needs to watch etcd and feed them into
	// this job control
	JobScheduled(jid string, host string, spec *JobSpec)
	JobDowned(jid string, host string, spec *JobSpec)
	HostDown(host string)
	HostUp(host string)
}

// A particular job with the user and host under which it runs
type JobWithHost struct {
	Spec *JobSpec
	Host string
	Jid  string
}

// An agent knows how to start and run a job. Each host in the cluster runs an agent
type HostAgent interface {
	// Agent has the right to refuse to run the job. It needs to check again
	// that job spec is satisfied
	RunJob(jid string, spec *JobSpec) error
}

// Knows the specs of all the machines in the cluster
type MachineDB interface {
	Spec(host string) (*MachineSpec, error)
}

// This interface specifies what job control will ask etcd
type Etcd interface {
	// Give me all the currently active hosts
	// (hosts that have an agent running, maintaining heartbeat with etcd)
	AllHosts() ([]string, error)
	// Give me all the jobs running in the cluster right now
	AllJobs() ([]*JobWithHost, error)
	// Give me an agent for the specified host
	HostAgent(host string) (HostAgent, error)
}
