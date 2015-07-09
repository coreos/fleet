// Copyright 2015 CoreOS, Inc.
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
	"reflect"
	"testing"
)

func TestStringSlice(t *testing.T) {
	tests := []struct {
		input  string
		output []string
	}{
		{`["a"]`, []string{"a"}},
		{`["a","b"]`, []string{"a", "b"}},
	}

	for _, tt := range tests {
		var ss StringSlice
		ss.Set(tt.input)
		r := ss.Value()
		if !reflect.DeepEqual(r, tt.output) {
			t.Errorf("error setting StringSlice: expected %+v, got %+v", tt.output, r)
		}
	}
}
