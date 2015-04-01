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

	"github.com/coreos/flt/functional/util"
	"github.com/coreos/flt/version"
)

func TestClientVersionFlag(t *testing.T) {
	stdout, _, err := util.RunFltctl("--version")
	if err != nil {
		t.Fatalf("Unexpected error while executing fltctl: %v", err)
	}

	if strings.TrimSpace(stdout) != fmt.Sprintf("fltctl version %s", version.Version) {
		t.Fatalf("Received unexpected output for `fltctl --version`: '%s'", stdout)
	}
}

func TestClientVersionHelpOutput(t *testing.T) {
	stdout, _, err := util.RunFltctl()
	if err != nil {
		t.Fatalf("Unexpected error while executing fltctl: %v", err)
	}

	if !strings.Contains(stdout, fmt.Sprintf("%s", version.Version)) {
		t.Fatalf("Could not find expected version string (%s) in help output:\n%s", version.Version, stdout)
	}
}

func TestClientHelpFlag(t *testing.T) {
	var err error
	var fixture, stdout, stderr string
	for i, tt := range []string{"--help", "-h", "help", ""} {
		if tt == "" {
			stdout, stderr, err = util.RunFltctl()
		} else {
			stdout, stderr, err = util.RunFltctl(tt)
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
			fixture = stdout
			continue
		}

		if stdout != fixture {
			t.Errorf("case %d: stdout:\n%s\n\ndiffers from stdout of case 0:\n%s", i, stdout, fixture)
		}
	}
}
