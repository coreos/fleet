package job

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/fleet/unit"
)

type JobState string

const (
	JobStateInactive = JobState("inactive")
	JobStateLoaded   = JobState("loaded")
	JobStateLaunched = JobState("launched")
)

// fleet-specific unit file requirement keys.
// "X-" prefix only appears in unit file and is dropped in code before the value is used.
const (
	// Require the unit be scheduled to a specific machine identified by given ID.
	fleetXConditionMachineID = "ConditionMachineID"
	// Legacy form of FleetXConditionMachineID.
	fleetXConditionMachineBootID = "ConditionMachineBootID"
	// Limit eligible machines to the one that hosts a specific unit.
	fleetXConditionMachineOf = "ConditionMachineOf"
	// Prevent a unit from being collocated with other units using glob-matching on the other unit names.
	fleetXConflicts = "Conflicts"
	// Machine metadata key in the unit file, without the X- prefix
	fleetXConditionMachineMetadata = "ConditionMachineMetadata"
	// Machine metadata key for the deprecated `require` flag
	fleetFlagMachineMetadata = "MachineMetadata"
)

func ParseJobState(s string) *JobState {
	js := JobState(s)
	if js != JobStateInactive && js != JobStateLoaded && js != JobStateLaunched {
		return nil
	}
	return &js
}

type Job struct {
	Name      string
	State     *JobState
	Unit      unit.Unit
	UnitHash  unit.Hash
	UnitState *unit.UnitState
}

// NewJob creates a new Job based on the given name and Unit.
// The returned Job has a populated UnitHash and empty JobState and
// UnitState. nil is returned on failure.
func NewJob(name string, unit unit.Unit) *Job {
	return &Job{
		Name:      name,
		State:     nil,
		Unit:      unit,
		UnitHash:  unit.Hash(),
		UnitState: nil,
	}
}

func (j *Job) Requirements() map[string][]string {
	return j.Unit.Requirements()
}

// Conflicts returns a list of Job names that cannot be scheduled to the same
// machine as this Job.
func (j *Job) Conflicts() []string {
	conflicts, ok := j.Requirements()[fleetXConflicts]
	if ok {
		return conflicts
	} else {
		return make([]string, 0)
	}
}

// Peers returns a list of Job names that must be scheduled to the same
// machine as this Job.
func (j *Job) Peers() []string {
	peers, ok := j.Requirements()[fleetXConditionMachineOf]
	if !ok {
		return []string{}
	}
	return peers
}

// RequiredTarget determines whether or not this Job must be scheduled to
// a specific machine. If such a requirement exists, the first value returned
// represents the ID of such a machine, while the second value will be a bool
// true. If no requirement exists, an empty string along with a bool false
// will be returned.
func (j *Job) RequiredTarget() (string, bool) {
	requirements := j.Unit.Requirements()

	machIDs, ok := requirements[fleetXConditionMachineID]
	if ok && len(machIDs) != 0 {
		return machIDs[0], true
	}

	// Fall back to the legacy option if it exists. This is unlikely
	// to actually work as the user intends, but it's better to
	// prevent a job from starting that has a legacy requirement
	// than to ignore the requirement and let it start.
	bootIDs, ok := requirements[fleetXConditionMachineBootID]
	if ok && len(bootIDs) != 0 {
		return bootIDs[0], true
	}

	return "", false
}

// Type attempts to determine the Type of systemd unit that this Job contains, based on the suffix of the job name
func (j *Job) Type() (string, error) {
	for _, ut := range unit.SupportedUnitTypes() {
		if strings.HasSuffix(j.Name, fmt.Sprintf(".%s", ut)) {
			return ut, nil
		}
	}

	return "", errors.New(fmt.Sprintf("Unrecognized systemd unit %s", j.Name))
}

// RequiredTargetMetadata return all machine-related metadata from a Job's requirements
func (j *Job) RequiredTargetMetadata() map[string][]string {
	metadata := make(map[string][]string)
	for key, values := range j.Unit.Requirements() {
		// Deprecated syntax added to the metadata via the old `--require` flag.
		if strings.HasPrefix(key, fleetFlagMachineMetadata) {
			if len(values) == 0 {
				continue
			}

			metadata[key[15:]] = values
		} else if key == fleetXConditionMachineMetadata {
			for _, valuePair := range values {
				s := strings.Split(valuePair, "=")

				if len(s) != 2 {
					continue
				}

				if len(s[0]) == 0 || len(s[1]) == 0 {
					continue
				}

				var mValues []string
				if mv, ok := metadata[s[0]]; ok {
					mValues = mv
				}

				metadata[s[0]] = append(mValues, s[1])
			}
		}
	}

	return metadata
}
