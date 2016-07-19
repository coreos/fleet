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

package pkg

import (
	"os"
	"os/user"
	"strings"
	"testing"
)

var pathtests = []struct {
	in     string
	home   string
	prefix string
	suffix string
	full   string
	expand bool
}{
	{"~/a", "", "/", "/a", "", true},
	{"~", "", "/", "", "", true},
	{"~~", "", "", "", "~~", false},
	{"~/a", "/home/foo", "", "", "/home/foo/a", true},
}

// TestParseFilepath tests parsing filepath
func TestParseFilepath(t *testing.T) {
	for _, test := range pathtests {
		oldHome := os.Getenv("HOME")
		if err := os.Setenv("HOME", test.home); err != nil {
			t.Fatalf("Failed setting $HOME")
		}
		path := ParseFilepath(test.in)
		if test.full != "" && path != test.full {
			t.Fatalf("Failed parsing out %v for %v", test.full, test.in)
		}
		if path == test.in && test.expand {
			// ParseFilepath does nothing if there is
			// nothing to expand, or if User or $HOME
			// are unknown otherwise it is an error
			currentHome := os.Getenv("HOME")
			_, err := user.Current()
			if currentHome != "" || err == nil {
				t.Fatalf("Failed parsing %v where $HOME or User are known", test.in)
			} else {
				t.Logf("User or $HOME are unknown ignoring test path %v", test.in)
				continue
			}
		}

		if !strings.HasPrefix(path, test.prefix) {
			t.Errorf("Failed parsing out prefix %v for %v", test.prefix, test.in)
		}
		if !strings.HasSuffix(path, test.suffix) {
			t.Errorf("Failed parsing out suffix %v for %v", test.suffix, test.in)
		}
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Failed recovering $HOME")
		}
	}
}
