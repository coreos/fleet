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

package resource

import "reflect"
import "testing"

func TestSum(t *testing.T) {
	for i, tt := range []struct {
		in   []ResourceTuple
		want ResourceTuple
	}{
		{
			[]ResourceTuple{ResourceTuple{10, 24, 1024}},
			ResourceTuple{10, 24, 1024},
		},
		{
			[]ResourceTuple{ResourceTuple{10, 24, 1024}, ResourceTuple{10, 24, 1024}},
			ResourceTuple{20, 48, 2048},
		},
		{
			[]ResourceTuple{},
			ResourceTuple{0, 0, 0},
		},
	} {
		got := Sum(tt.in...)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %v, want %v", i, got, tt.want)
		}
	}
}

func TestSub(t *testing.T) {
	for i, tt := range []struct {
		r1   ResourceTuple
		r2   ResourceTuple
		want ResourceTuple
	}{
		{
			ResourceTuple{10, 24, 1024},
			ResourceTuple{10, 24, 1024},
			ResourceTuple{0, 0, 0},
		},
		{
			ResourceTuple{20, 48, 2048},
			ResourceTuple{15, 36, 2048},
			ResourceTuple{5, 12, 0},
		},
		{
			ResourceTuple{0, 0, 0},
			ResourceTuple{0, 0, 0},
			ResourceTuple{0, 0, 0},
		},
	} {
		got := Sub(tt.r1, tt.r2)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("case %d: got %v, want %v", i, got, tt.want)
		}
	}
}
