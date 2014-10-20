/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package platform

import (
	"strconv"

	"github.com/coreos/fleet/functional/util"
)

type Cluster interface {
	CreateMember(string, MachineConfig) error
	DestroyMember(string) error
	PoweroffMember(string) error
	Members() []string
	MemberCommand(string, ...string) (string, error)
	Destroy() error

	// client operations
	Fleetctl(args ...string) (string, string, error)
	FleetctlWithInput(input string, args ...string) (string, string, error)
	WaitForNActiveUnits(count int) (map[string][]util.UnitState, error)
	WaitForNMachines(count int) ([]string, error)
}

// MachineConfig defines the parameters that should
// be considered when creating a new cluster member.
type MachineConfig struct {
	VerifyUnits bool
}

func CreateNClusterMembers(cl Cluster, count int, cfg MachineConfig) error {
	for i := 0; i < count; i++ {
		name := strconv.Itoa(i)
		if err := cl.CreateMember(name, cfg); err != nil {
			return err
		}
	}
	return nil
}
