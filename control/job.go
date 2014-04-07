package control

import (
	"strconv"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

const (
	defaultMemoryRequired    = 512
	defaultCoresRequired     = 100
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

	if j.JobRequirements == nil {
		log.Errorf("missing mandatory requirements (memory, cores and disk): %s; filling in defaults", j.Name)

		spec.CoresRequired = defaultCoresRequired
		spec.MemoryRequired = defaultMemoryRequired
		spec.DiskSpaceRequired = defaultDiskSpaceRequired
		return spec
	}

	spec.ConflictsWith = j.JobRequirements[unit.FleetXConflicts]
	spec.DependsOn = j.JobRequirements[unit.FleetXConditionMachineOf]
	spec.RequiresHost = stringRequirement(unit.FleetXConditionMachineBootID, j.JobRequirements, "")

	mem, err := intRequirement(unit.FleetXMemoryRequired, j.JobRequirements, defaultMemoryRequired)
	if err != nil {
		log.Errorf("failed to parse FleetXMemoryRequired: %s; filling in defaults", j.Name)
		mem = defaultMemoryRequired
	}
	spec.MemoryRequired = mem

	cores, err := intRequirement(unit.FleetXCoresRequired, j.JobRequirements, defaultCoresRequired)
	if err != nil {
		log.Errorf("failed to parse FleetXCoresRequired: %s; filling in defaults", j.Name)
		cores = defaultCoresRequired
	}
	spec.CoresRequired = cores

	disk, err := intRequirement(unit.FleetXDiskSpaceRequired, j.JobRequirements, defaultDiskSpaceRequired)
	if err != nil {
		log.Errorf("failed to parse FleetXDiskSpaceRequired: %s; filling in defaults", j.Name)
		disk = defaultDiskSpaceRequired
	}
	spec.DiskSpaceRequired = disk

	return spec
}
