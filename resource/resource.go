// Copyright 2014 The fleet Authors
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

package resource

// ResourceTuple groups together CPU, memory and disk space. This could be
// total, available or consumed. It could also be used by job resource requirements.
type ResourceTuple struct {
	// in hundreds, ie 100=1core, 50=0.5core, 200=2cores, etc
	Cores int
	// in MB
	Memory int
	// in MB
	Disk int
}

// Empty returns true if all components of the ResourceTuple are zero.
func (rt ResourceTuple) Empty() bool {
	return rt.Cores == 0 && rt.Memory == 0 && rt.Disk == 0
}

const (
	// TODO(jonboulle): make these configurable
	HostCores  = 100
	HostMemory = 256
	HostDisk   = 0
)

// HostResources represents a set of resources that fleet considers reserved
// for the host, i.e. outside of any units it is running
var HostResources = ResourceTuple{
	HostCores,
	HostMemory,
	HostDisk,
}

// Sum aggregates a number of ResourceTuples into a single entity
func Sum(resources ...ResourceTuple) (res ResourceTuple) {
	for _, r := range resources {
		res.Cores += r.Cores
		res.Memory += r.Memory
		res.Disk += r.Disk
	}
	return
}

// Sub returns a ResourceTuple representing the difference between two
// ResourceTuples
func Sub(r1, r2 ResourceTuple) (res ResourceTuple) {
	res.Cores = r1.Cores - r2.Cores
	res.Memory = r1.Memory - r2.Memory
	res.Disk = r1.Disk - r2.Disk
	return
}
