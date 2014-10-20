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

package unit

import (
	"reflect"
	"testing"
)

func TestNewSystemdUnitFileFromLegacyContents(t *testing.T) {
	legacy := map[string]map[string]string{
		"Unit": {
			"Description": "foobar",
		},
		"Service": {
			"Type":      "oneshot",
			"ExecStart": "/usr/bin/echo bar",
		},
	}

	expected := map[string]map[string][]string{
		"Unit": {
			"Description": {"foobar"},
		},
		"Service": {
			"Type":      {"oneshot"},
			"ExecStart": {"/usr/bin/echo bar"},
		},
	}

	u, err := NewUnitFromLegacyContents(legacy)
	if err != nil {
		t.Fatalf("Unexpected error parsing unit %q: %v", legacy, err)
	}
	actual := u.Contents

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Map func did not produce expected output.\nActual=%v\nExpected=%v", actual, expected)
	}
}
