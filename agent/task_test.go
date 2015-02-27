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

package agent

import (
	"reflect"
	"sort"
	"testing"
)

func TestTaskSorting(t *testing.T) {
	tests := []struct {
		tasks []task
		want  []task
	}{
		// Unload before Load
		{
			tasks: []task{
				task{typ: taskTypeLoadUnit, reason: "A"},
				task{typ: taskTypeUnloadUnit, reason: "B"},
			},
			want: []task{
				task{typ: taskTypeUnloadUnit, reason: "B"},
				task{typ: taskTypeLoadUnit, reason: "A"},
			},
		},

		// Load before Stop
		{
			tasks: []task{
				task{typ: taskTypeStopUnit, reason: "A"},
				task{typ: taskTypeLoadUnit, reason: "B"},
			},
			want: []task{
				task{typ: taskTypeLoadUnit, reason: "B"},
				task{typ: taskTypeStopUnit, reason: "A"},
			},
		},

		// Stop before Start
		{
			tasks: []task{
				task{typ: taskTypeStartUnit, reason: "A"},
				task{typ: taskTypeStopUnit, reason: "B"},
			},
			want: []task{
				task{typ: taskTypeStopUnit, reason: "B"},
				task{typ: taskTypeStartUnit, reason: "A"},
			},
		},

		// Relative ordering preserved when equivalent tasks present
		{
			tasks: []task{
				task{typ: taskTypeLoadUnit, reason: "A"},
				task{typ: taskTypeStartUnit, reason: "A"},
				task{typ: taskTypeLoadUnit, reason: "B"},
				task{typ: taskTypeStartUnit, reason: "B"},
			},
			want: []task{
				task{typ: taskTypeLoadUnit, reason: "A"},
				task{typ: taskTypeLoadUnit, reason: "B"},
				task{typ: taskTypeStartUnit, reason: "A"},
				task{typ: taskTypeStartUnit, reason: "B"},
			},
		},
	}

	for i, tt := range tests {
		got := make([]task, len(tt.tasks))
		copy(got, tt.tasks)
		sort.Sort(sortableTasks(got))
		if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("case %d: want=%#v got=%#v", i, tt.want, got)
		}
	}
}
