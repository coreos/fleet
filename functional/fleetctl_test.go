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

package functional

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coreos/fleet/functional/util"
	"github.com/coreos/fleet/version"
)

func TestClientVersionFlag(t *testing.T) {
	stdout, stderr, err := util.RunFleetctl("version")
	if err != nil {
		t.Fatalf("Unexpected error while executing fleetctl:\nstdout: %s\nstderr: %s\nerr: %v",
			stdout, stderr, err)
	}

	if strings.TrimSpace(stdout) != fmt.Sprintf("fleetctl version %s", version.Version) {
		t.Fatalf("Received unexpected output for `fleetctl --version`: '%s'", stdout)
	}
}

func TestClientVersionHelpOutput(t *testing.T) {
	stdout, stderr, err := util.RunFleetctl("help")
	if err != nil {
		t.Fatalf("Unexpected error while executing fleetctl:\nstdout: %s\nstderr: %s\nerr: %v",
			stdout, stderr, err)
	}

	if !strings.Contains(stdout, fmt.Sprintf("%s", version.Version)) {
		t.Fatalf("Could not find expected version string (%s) in help output:\n%s", version.Version, stdout)
	}
}

func TestClientHelpFlag(t *testing.T) {
	var err error
	var stdout, stderr string
	for i, tt := range []string{"--help", "-h", "help", ""} {
		if tt == "" {
			stdout, stderr, err = util.RunFleetctl()
		} else {
			stdout, stderr, err = util.RunFleetctl(tt)
		}

		if err != nil {
			t.Fatalf("case %d: failed getting %s output: %v\n\nstdout: %s\n\nstderr: %s", i, tt, err, stdout, stderr)
		}

		// use the output of the first test case as the point
		// of comparison for all future cases
		if i == 0 {
			if len(stdout) == 0 {
				t.Fatalf("case 0: initial case has no help output")
			}
			continue
		}
	}
}
