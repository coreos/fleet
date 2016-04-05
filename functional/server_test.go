// Copyright 2016 CoreOS, Inc.
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

package functional

import (
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/platform"
)

// TestReconfigureServer checks whether fleetd managed to keep its listeners
// across reconfiguration of fleetd after receiving SIGHUP.
func TestReconfigureServer(t *testing.T) {
	cluster, err := platform.NewNspawnCluster("smoke")
	if err != nil {
		t.Fatal(err)
	}
	defer cluster.Destroy()

	m0, err := cluster.CreateMember()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cluster.WaitForNMachines(m0, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Check expected state before reconfiguring fleetd
	stdout, _ := cluster.MemberCommand(m0, "systemctl", "show", "--property=ActiveState", "fleet")
	if strings.TrimSpace(stdout) != "ActiveState=active" {
		t.Fatalf("Fleet unit not reported as active: %s", stdout)
	}
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=Result", "fleet")
	if strings.TrimSpace(stdout) != "Result=success" {
		t.Fatalf("Result for fleet unit not reported as success: %s", stdout)
	}

	// NOTE: we need to sleep once here to get reliable results.
	// Without this sleep, the entire fleetd test always ends up succeeding
	// no matter whether SIGHUP came or not.
	_, _ = cluster.MemberCommand(m0, "sh", "-c", `'sleep 2'`)

	stdout, _ = cluster.MemberCommand(m0, "sudo", "kill", "-HUP", "$(pidof fleetd)")
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("Sending SIGHUP to fleetd returned: %s", stdout)
	}

	// Check for HTTP listener error looking into the fleetd journal
	stdout, _ = cluster.MemberCommand(m0, "sh", "-c",
		`'journalctl -u fleet | grep "Failed serving HTTP on listener:"'`)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("Fleetd log returned error on HTTP listeners: %s", stdout)
	}

	// Check expected state after reconfiguring fleetd
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=ActiveState", "fleet")
	if strings.TrimSpace(stdout) != "ActiveState=active" {
		t.Fatalf("Fleet unit not reported as active: %s", stdout)
	}
	stdout, _ = cluster.MemberCommand(m0, "systemctl", "show", "--property=Result", "fleet")
	if strings.TrimSpace(stdout) != "Result=success" {
		t.Fatalf("Result for fleet unit not reported as success: %s", stdout)
	}
}
