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
