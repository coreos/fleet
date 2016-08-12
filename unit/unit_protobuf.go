package unit

import (
	pb "github.com/coreos/fleet/protobuf"
)

func (unitFile *UnitFile) ToPB() pb.UnitFile {
	options := make([]pb.UnitOption, len(unitFile.Options))
	for i, opt := range unitFile.Options {
		options[i] = pb.UnitOption{
			Name:    opt.Name,
			Section: opt.Section,
			Value:   opt.Value,
		}
	}

	return pb.UnitFile{UnitOptions: options}
}

func (s UnitState) ToPB() *pb.UnitState {
	return &pb.UnitState{
		Name:        s.UnitName,
		Hash:        s.UnitHash,
		LoadState:   s.LoadState,
		ActiveState: s.ActiveState,
		SubState:    s.SubState,
		MachineID:   s.MachineID,
	}
}
