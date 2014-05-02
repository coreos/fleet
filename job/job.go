package job

import (
	"errors"
	"fmt"
	"github.com/coreos/fleet/unit"
	"strings"
)

type JobState string

const (
	JobStateInactive = JobState("inactive")
	JobStateLoaded   = JobState("loaded")
	JobStateLaunched = JobState("launched")
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

func (self *Job) Requirements() map[string][]string {
	return self.Unit.Requirements()
}

// Peers returns a list of Job names that must be scheduled to the same
// machine as this Job. If no peers were explicitly defined for certain unit
// types, a default list of peers will be returned. This behavior only applies
// to the socket unit type. For example, the default peer of foo.socket would
// be foo.service.
func (self *Job) Peers() []string {
	if peers, ok := self.Requirements()[unit.FleetXConditionMachineOf]; ok {
		return peers
	}

	peers := make([]string, 0)

	jType, err := self.Type()
	if err != nil {
		return peers
	}

	if jType != "socket" {
		return peers
	}

	baseName := strings.TrimSuffix(self.Name, fmt.Sprintf(".%s", jType))
	serviceName := fmt.Sprintf("%s.%s", baseName, "service")
	peers = append(peers, serviceName)

	return peers
}

// RequiredTarget determines whether or not this Job must be scheduled to
// a specific machine. If such a requirement exists, the first value returned
// represents the ID of such a machine, while the second value will be a bool
// true. If no requirement exists, an empty string along with a bool false
// will be returned.
func (self *Job) RequiredTarget() (string, bool) {
	requirements := self.Unit.Requirements()

	bootIDs, ok := requirements[unit.FleetXConditionMachineBootID]
	if ok && len(bootIDs) != 0 {
		return bootIDs[0], true
	}

	return "", false
}

// Type attempts to determine the Type of systemd unit that this Job contains, based on the suffix of the job name
func (self *Job) Type() (string, error) {
	for _, ut := range unit.SupportedUnitTypes() {
		if strings.HasSuffix(self.Name, fmt.Sprintf(".%s", ut)) {
			return ut, nil
		}
	}

	return "", errors.New(fmt.Sprintf("Unrecognized systemd unit %s", self.Name))
}
