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

package platform

import (
	"testing"

	"github.com/coreos/fleet/functional/util"
)

type Member interface {
	ID() string
	IP() string
	Endpoint() string
}

type Cluster interface {
	CreateMember() (Member, error)
	DestroyMember(Member) error
	ReplaceMember(Member) (Member, error)
	Members() []Member
	MemberCommand(Member, ...string) (string, error)
	Destroy(t *testing.T) error

	// client operations
	Fleetctl(m Member, args ...string) (string, string, error)
	FleetctlWithInput(m Member, input string, args ...string) (string, string, error)
	FleetctlWithEnv(m Member, args ...string) (string, string, error)
	WaitForNUnits(Member, int, ...string) (map[string][]util.UnitState, error)
	WaitForNActiveUnits(Member, int, ...string) (map[string][]util.UnitState, error)
	WaitForNUnitFiles(Member, int, ...string) (map[string][]util.UnitFileState, error)
	WaitForNMachines(Member, int) ([]string, error)
}

func CreateNClusterMembers(cl Cluster, count int) ([]Member, error) {
	ms := make([]Member, 0)
	for i := 0; i < count; i++ {
		m, err := cl.CreateMember()
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}
