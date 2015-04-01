// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package job

import (
	"fmt"
	"strings"

	"github.com/coreos/flt/pkg"
	"github.com/coreos/flt/unit"
)

type JobState string

const (
	JobStateInactive = JobState("inactive")
	JobStateLoaded   = JobState("loaded")
	JobStateLaunched = JobState("launched")
)

// flt-specific unit file requirement keys.
// For certain values, the (optional, deprecated) "X-" or "X-Condition"
// prefixes appear in unit files but are dropped in code before the value is used.
const (
	// Require the unit be scheduled to a specific machine identified by given ID.
	fltMachineID = "MachineID"
	// Legacy form of fltMachineID.
	fltMachineBootID = "MachineBootID"
	// Limit eligible machines to the one that hosts a specific unit.
	fltMachineOf = "MachineOf"
	// Prevent a unit from being collocated with other units using glob-matching on the other unit names.
	fltConflicts = "Conflicts"
	// Machine metadata key in the unit file
	fltMachineMetadata = "MachineMetadata"
	// Require that the unit be scheduled on every machine in the cluster
	fltGlobal = "Global"

	deprecatedXPrefix          = "X-"
	deprecatedXConditionPrefix = "X-Condition"
)

// validRequirements encapsulates all current and deprecated unit file requirement keys
var validRequirements = pkg.NewUnsafeSet(
	fltMachineID,
	deprecatedXConditionPrefix+fltMachineID,
	deprecatedXConditionPrefix+fltMachineBootID,
	deprecatedXConditionPrefix+fltMachineOf,
	fltMachineOf,
	deprecatedXPrefix+fltConflicts,
	fltConflicts,
	deprecatedXConditionPrefix+fltMachineMetadata,
	fltMachineMetadata,
	fltGlobal,
)

func ParseJobState(s string) (JobState, error) {
	js := JobState(s)

	var err error
	if js != JobStateInactive && js != JobStateLoaded && js != JobStateLaunched {
		err = fmt.Errorf("invalid value %q for JobState", s)
		js = JobStateInactive
	}

	return js, err
}

// Job is a legacy construct encapsulating a scheduled unit in flt
type Job struct {
	Name            string
	State           *JobState
	TargetState     JobState
	TargetMachineID string
	Unit            unit.UnitFile
}

// ScheduledUnit represents a Unit known by flt and encapsulates its current scheduling state. This does not include Global units.
type ScheduledUnit struct {
	Name            string
	State           *JobState
	TargetMachineID string
}

// Unit represents a Unit that has been submitted to flt
// (list-unit-files)
type Unit struct {
	Name        string
	Unit        unit.UnitFile
	TargetState JobState
}

// IsGlobal returns whether a Unit is considered a global unit
func (u *Unit) IsGlobal() bool {
	j := &Job{
		Name: u.Name,
		Unit: u.Unit,
	}
	values := j.requirements()[fltGlobal]
	if len(values) == 0 {
		return false
	}
	// Last value found wins
	last := values[len(values)-1]
	return strings.ToLower(last) == "true"
}

// NewJob creates a new Job based on the given name and Unit.
// The returned Job has a populated UnitHash and empty JobState.
// nil is returned on failure.
func NewJob(name string, unit unit.UnitFile) *Job {
	return &Job{
		Name:            name,
		State:           nil,
		TargetState:     JobStateInactive,
		TargetMachineID: "",
		Unit:            unit,
	}
}

// The following helper functions are to facilitate the transition from Job --> Unit
func (u *Unit) Conflicts() []string {
	j := &Job{
		Name: u.Name,
		Unit: u.Unit,
	}
	return j.Conflicts()
}

func (u *Unit) Peers() []string {
	j := &Job{
		Name: u.Name,
		Unit: u.Unit,
	}
	return j.Peers()
}

func (u *Unit) RequiredTarget() (string, bool) {
	j := &Job{
		Name: u.Name,
		Unit: u.Unit,
	}
	return j.RequiredTarget()
}

func (u *Unit) RequiredTargetMetadata() map[string]pkg.Set {
	j := &Job{
		Name: u.Name,
		Unit: u.Unit,
	}
	return j.RequiredTargetMetadata()
}

// requirements returns all relevant options from the [X-Flt] section of a unit file.
// Relevant options are identified with a `X-` prefix in the unit.
// This prefix is stripped from relevant options before being returned.
// Furthermore, specifier substitution (using unitPrintf) is performed on all requirements.
func (j *Job) requirements() map[string][]string {
	uni := unit.NewUnitNameInfo(j.Name)
	requirements := make(map[string][]string)
	for key, values := range j.Unit.Contents["X-Flt"] {
		if _, ok := requirements[key]; !ok {
			requirements[key] = make([]string, 0)
		}

		if uni != nil {
			for i, v := range values {
				values[i] = unitPrintf(v, *uni)
			}
		}
		requirements[key] = values
	}

	return requirements
}

// ValidateRequirements ensures that all options in the [X-Flt] section of
// the job's associated unit file are known keys. If not, an error is
// returned.
func (j *Job) ValidateRequirements() error {
	for key, _ := range j.requirements() {
		if !validRequirements.Contains(key) {
			return fmt.Errorf("unrecognized requirement in [X-Flt] section: %q", key)
		}
	}
	return nil
}

// Conflicts returns a list of Job names that cannot be scheduled to the same
// machine as this Job.
func (j *Job) Conflicts() []string {
	conflicts := make([]string, 0)
	conflicts = append(conflicts, j.requirements()[deprecatedXPrefix+fltConflicts]...)
	conflicts = append(conflicts, j.requirements()[fltConflicts]...)
	return conflicts
}

// Peers returns a list of Job names that must be scheduled to the same
// machine as this Job.
func (j *Job) Peers() []string {
	peers := make([]string, 0)
	peers = append(peers, j.requirements()[deprecatedXConditionPrefix+fltMachineOf]...)
	peers = append(peers, j.requirements()[fltMachineOf]...)
	return peers
}

// RequiredTarget determines whether or not this Job must be scheduled to
// a specific machine. If such a requirement exists, the first value returned
// represents the ID of such a machine, while the second value will be a bool
// true. If no requirement exists, an empty string along with a bool false
// will be returned.
func (j *Job) RequiredTarget() (string, bool) {
	requirements := j.requirements()

	var machIDs []string
	var ok bool
	// Best case: look for modern declaration
	machIDs, ok = requirements[fltMachineID]
	if ok && len(machIDs) != 0 {
		return machIDs[0], true
	}

	// First fall back to the deprecated syntax
	machIDs, ok = requirements[deprecatedXConditionPrefix+fltMachineID]
	if ok && len(machIDs) != 0 {
		return machIDs[0], true
	}

	// Finally, fall back to the legacy option if it exists. This is
	// unlikely to actually work as the user intends, but it's better to
	// prevent a job from starting that has a legacy requirement than to
	// ignore the requirement and let it start.
	bootIDs, ok := requirements[deprecatedXConditionPrefix+fltMachineBootID]
	if ok && len(bootIDs) != 0 {
		return bootIDs[0], true
	}

	return "", false
}

// RequiredTargetMetadata return all machine-related metadata from a Job's
// requirements. Valid metadata fields are strings of the form `key=value`,
// where both key and value are not the empty string.
func (j *Job) RequiredTargetMetadata() map[string]pkg.Set {
	metadata := make(map[string]pkg.Set)

	for _, key := range []string{
		deprecatedXConditionPrefix + fltMachineMetadata,
		fltMachineMetadata,
	} {
		for _, valuePair := range j.requirements()[key] {
			s := strings.Split(valuePair, "=")

			if len(s) != 2 {
				continue
			}

			if len(s[0]) == 0 || len(s[1]) == 0 {
				continue
			}

			if _, ok := metadata[s[0]]; !ok {
				metadata[s[0]] = pkg.NewUnsafeSet()
			}
			metadata[s[0]].Add(s[1])
		}
	}

	return metadata
}

func (j *Job) Scheduled() bool {
	return len(j.TargetMachineID) > 0
}

// unitPrintf is analogous to systemd's `unit_name_printf`. It will take the
// given string and replace the following specifiers with the values from the
// provided UnitNameInfo:
// 	%n: the full name of the unit               (foo@bar.waldo)
// 	%N: the name of the unit without the suffix (foo@bar)
// 	%p: the prefix                              (foo)
// 	%i: the instance                            (bar)
func unitPrintf(s string, nu unit.UnitNameInfo) (out string) {
	out = strings.Replace(s, "%n", nu.FullName, -1)
	out = strings.Replace(out, "%N", nu.Name, -1)
	out = strings.Replace(out, "%p", nu.Prefix, -1)
	out = strings.Replace(out, "%i", nu.Instance, -1)
	return
}
