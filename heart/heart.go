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

package heart

import (
	"time"

	"github.com/coreos/flt/machine"
	"github.com/coreos/flt/registry"
)

type Heart interface {
	Beat(time.Duration) (uint64, error)
	Clear() error
}

func New(reg registry.Registry, mach machine.Machine) Heart {
	return &machineHeart{reg, mach}
}

type machineHeart struct {
	reg  registry.Registry
	mach machine.Machine
}

func (h *machineHeart) Beat(ttl time.Duration) (uint64, error) {
	return h.reg.SetMachineState(h.mach.State(), ttl)
}

func (h *machineHeart) Clear() error {
	return h.reg.RemoveMachineState(h.mach.State().ID)
}
