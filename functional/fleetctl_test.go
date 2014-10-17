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

package functional

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/util"
	"github.com/coreos/fleet/version"
)

func TestClientVersionFlag(t *testing.T) {
	stdout, _, err := util.RunFleetctl("--version")
	if err != nil {
		t.Fatalf("Unexpected error while executing fleetctl: %v", err)
	}

	if strings.TrimSpace(stdout) != fmt.Sprintf("fleetctl version %s", version.Version) {
		t.Fatalf("Received unexpected output for `fleetctl --version`: '%s'", stdout)
	}
}

func TestClientVersionHelpOutput(t *testing.T) {
	stdout, _, err := util.RunFleetctl()
	if err != nil {
		t.Fatalf("Unexpected error while executing fleetctl: %v", err)
	}

	if !strings.Contains(stdout, fmt.Sprintf("%s", version.Version)) {
		t.Fatalf("Could not find expected version string (%s) in help output:\n%s", version.Version, stdout)
	}
}
