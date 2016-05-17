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

package engine

import (
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func TestSchedulerDecisions(t *testing.T) {
	tests := []struct {
		clust *clusterState
		job   *job.Job
		dec   *decision
	}{
		// no machines to receive job
		{
			clust: newClusterState([]job.Unit{}, []job.ScheduledUnit{}, []machine.MachineState{}),
			job:   &job.Job{Name: "foo.service"},
			dec:   nil,
		},

		// multiple machines, pick the first one
		{
			clust: newClusterState([]job.Unit{}, []job.ScheduledUnit{}, []machine.MachineState{machine.MachineState{ID: "XXX"}, machine.MachineState{ID: "YYY"}}),
			job:   &job.Job{Name: "foo.service"},
			dec: &decision{
				machineID: "XXX",
			},
		},
	}

	for i, tt := range tests {
		sched := &leastLoadedScheduler{}
		dec, err := sched.Decide(tt.clust, tt.job)

		if err != nil && tt.dec != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
			continue
		} else if err == nil && tt.dec == nil {
			t.Errorf("case %d: expected error", i)
			continue
		}

		if !reflect.DeepEqual(tt.dec, dec) {
			t.Errorf("case %d: expected decision %#v, got %#v", i, tt.dec, dec)
		}
	}
}

func buildUnitWithWeight(shares int) *job.Unit {
	return &job.Unit{
		Unit: unit.UnitFile{
			Contents: map[string]map[string][]string{
				"X-Fleet": map[string][]string{
					"Weight": []string{strconv.Itoa(shares)},
				},
			},
		},
	}
}

func TestAgentStateSorting(t *testing.T) {
	Unit6th := buildUnitWithWeight(10)
	Unit7th := buildUnitWithWeight(20)

	tests := []struct {
		in  []*agent.AgentState
		out []*agent.AgentState
	}{
		{
			in:  []*agent.AgentState{},
			out: []*agent.AgentState{},
		},

		// sort by number of jobs scheduled to agent
		{
			in: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{ID: "A"},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
						"5.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{ID: "B"},
					Units: map[string]*job.Unit{
						"6.service": &job.Unit{},
						"7.service": &job.Unit{},
					},
				},
			},
			out: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{ID: "B"},
					Units: map[string]*job.Unit{
						"6.service": &job.Unit{},
						"7.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{ID: "A"},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
						"5.service": &job.Unit{},
					},
				},
			},
		},

		// sort by weigth of jobs scheduled to agent
		{
			in: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{ID: "A"},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
						"5.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{ID: "B"},
					Units: map[string]*job.Unit{
						"6.service": Unit6th,
						"7.service": Unit7th,
					},
				},
			},
			out: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{ID: "AAA"},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
						"5.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{ID: "BBB"},
					Units: map[string]*job.Unit{
						"6.service": Unit6th,
						"7.service": Unit7th,
					},
				},
			},
		},

		// fall back to sorting by shares  when # jobs is equal
		{
			in: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "B",
						Metadata: map[string]string{"shares": "2"},
					},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "A",
						Metadata: map[string]string{"shares": "3"},
					},
					Units: map[string]*job.Unit{
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
					},
				},
			},
			out: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "A",
						Metadata: map[string]string{"shares": "3"},
					},
					Units: map[string]*job.Unit{
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "B",
						Metadata: map[string]string{"shares": "2"},
					},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
					},
				},
			},
		},

		// fall back to sorting alphabetically by machine ID when # jobs is equal
		{
			in: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{ID: "B"},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{ID: "A"},
					Units: map[string]*job.Unit{
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
					},
				},
			},
			out: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{ID: "A"},
					Units: map[string]*job.Unit{
						"3.service": &job.Unit{},
						"4.service": &job.Unit{},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{ID: "B"},
					Units: map[string]*job.Unit{
						"1.service": &job.Unit{},
						"2.service": &job.Unit{},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		sortable := make(sortableAgentStates, len(tt.in))
		for i, ms := range tt.in {
			ms := ms
			sortable[i] = ms
		}

		sort.Sort(sortable)
		sorted := []*agent.AgentState(sortable)

		if !reflect.DeepEqual(tt.out, sorted) {
			t.Errorf("case %d: unexpected output", i)
			for ii, ms := range tt.out {
				t.Logf("case %d: tt.out[%d] = %#v", i, ii, *ms)
			}
			for ii, ms := range sorted {
				t.Logf("case %d: sorted[%d] = %#v", i, ii, *ms)
			}
		}
	}
}
