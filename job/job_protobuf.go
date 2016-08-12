package job

import (
	pb "github.com/coreos/fleet/protobuf"
)

func (u *Unit) ToPB() pb.Unit {
	return pb.Unit{
		Name:         u.Name,
		Unit:         u.Unit.ToPB(),
		DesiredState: u.TargetState.ToPB(),
	}
}

func (u *ScheduledUnit) ToPB() pb.ScheduledUnit {
	unit := pb.ScheduledUnit{
		Name:         u.Name,
		CurrentState: u.State.ToPB(),
		MachineID:    u.TargetMachineID,
	}
	return unit
}

func (state JobState) ToPB() pb.TargetState {
	switch state {
	case JobStateInactive:
		return pb.TargetState_INACTIVE
	case JobStateLoaded:
		return pb.TargetState_LOADED
	case JobStateLaunched:
		return pb.TargetState_LAUNCHED
	}
	return pb.TargetState_LOADED
}
