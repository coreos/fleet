package control

import (
	"strconv"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
)

const (
	defaultMemoryRequired    = 10
	defaultCoresRequired     = 10
	defaultDiskSpaceRequired = 1024
)

func stringRequirement(key string, jr map[string][]string, defaultValue string) string {
	vs := jr[key]
	if len(vs) == 0 {
		return defaultValue
	}
	return vs[0]
}

func intRequirement(key string, jr map[string][]string, defaultValue int) (int, error) {
	vs := jr[key]
	if len(vs) == 0 {
		return defaultValue, nil
	}
	v, err := strconv.Atoi(vs[0])
	if err != nil {
		return 0, err
	}
	return v, nil
}

func JobSpecFrom(j *job.Job) *JobSpec {
	spec := new(JobSpec)
	spec.Name = j.Name

	jobRequirements := j.Requirements()

	spec.ConflictsWith = jobRequirements[job.FleetXConflicts]
	spec.DependsOn = jobRequirements[job.FleetXConditionMachineOf]
	spec.RequiresHost = stringRequirement(job.FleetXConditionMachineBootID, jobRequirements, "")

	mem, err := intRequirement(job.FleetXMemoryRequired, jobRequirements, defaultMemoryRequired)
	if err != nil {
		log.Errorf("failed to parse FleetXMemoryRequired: %s; filling in defaults", j.Name)
		mem = defaultMemoryRequired
	}
	spec.MemoryRequired = mem

	cores, err := intRequirement(job.FleetXCoresRequired, jobRequirements, defaultCoresRequired)
	if err != nil {
		log.Errorf("failed to parse FleetXCoresRequired: %s; filling in defaults", j.Name)
		cores = defaultCoresRequired
	}
	spec.CoresRequired = cores

	disk, err := intRequirement(job.FleetXDiskSpaceRequired, jobRequirements, defaultDiskSpaceRequired)
	if err != nil {
		log.Errorf("failed to parse FleetXDiskSpaceRequired: %s; filling in defaults", j.Name)
		disk = defaultDiskSpaceRequired
	}
	spec.DiskSpaceRequired = disk

	return spec
}
