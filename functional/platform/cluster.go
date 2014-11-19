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

type Member interface {
	ID() string
	IP() string
}

type Cluster interface {
	CreateMember(string) (Member, error)
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

func CreateNClusterMembers(cl Cluster, count int) error {
	for i := 0; i < count; i++ {
		name := strconv.Itoa(i)
		if _, err := cl.CreateMember(name); err != nil {
			return err
		}
	}
	return nil
}
