// Copyright 2016 The fleet Authors
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

package rpc

import (
	sdunit "github.com/coreos/go-systemd/unit"

	"github.com/coreos/fleet/job"
	pb "github.com/coreos/fleet/protobuf"
	"github.com/coreos/fleet/unit"
)

func rpcUnitStateToJobState(state pb.TargetState) job.JobState {
	switch state {
	case pb.TargetState_INACTIVE:
		return job.JobStateInactive
	case pb.TargetState_LOADED:
		return job.JobStateLoaded
	case pb.TargetState_LAUNCHED:
		return job.JobStateLaunched
	}
	return job.JobStateInactive
}

func rpcUnitStateToExtUnitState(state *pb.UnitState) *unit.UnitState {
	return &unit.UnitState{
		UnitName:    state.Name,
		UnitHash:    state.Hash,
		LoadState:   state.LoadState,
		ActiveState: state.ActiveState,
		SubState:    state.SubState,
		MachineID:   state.MachineID,
	}
}

func rpcUnitToJobUnit(u *pb.Unit) *job.Unit {
	unitOptions := make([]*sdunit.UnitOption, len(u.Unit.UnitOptions))

	for i, option := range u.Unit.UnitOptions {
		unitOptions[i] = &sdunit.UnitOption{
			Section: option.Section,
			Name:    option.Name,
			Value:   option.Value,
		}
	}

	return &job.Unit{
		Name:        u.Name,
		Unit:        *unit.NewUnitFromOptions(unitOptions),
		TargetState: rpcUnitStateToJobState(u.DesiredState),
	}
}
