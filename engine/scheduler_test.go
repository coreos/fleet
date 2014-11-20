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

package engine

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/unit"
)

func newUnitWithSchedulerMetadata(t *testing.T, scheduler, metadata string) unit.UnitFile {
	contents := fmt.Sprintf("[X-Fleet]\nScheduler=%s\nSchedulerMetadata=%s", scheduler, metadata)
	u, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("error creating unit from %q: %v", contents, err)
	}
	return *u
}

func TestSchedulerSelection(t *testing.T) {

	sched := &selectingScheduler{
		map[string]Scheduler{
			"test":  &fakeSpecMachineIdScheduler{"test"},
			"test2": &fakeSpecMachineIdScheduler{"test2"},
		},
	}

	tests := []struct {
		job *job.Job
		dec *decision
	}{

		// choose test Scheduler
		{
			job: &job.Job{
				Name: "foo.service",
				Unit: newUnitWithSchedulerMetadata(t, "test", ""),
			},
			dec: &decision{
				machineID: "test",
			},
		},
	}

	for i, tt := range tests {
		dec, err := sched.Decide(nil, tt.job)

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

func TestDefaultSchedulerDecisions(t *testing.T) {
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
		sched := &selectingScheduler{}
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

func TestAgentStateSorting(t *testing.T) {

	_leastLoadedSorter := &leastLoadedSorter{}
	_leastSystemStatFieldSorter := &leastSystemStatFieldSorter{}

	tests := []struct {
		sorter agentSorter
		job    *job.Job
		in     []*agent.AgentState
		out    []*agent.AgentState
	}{
		{
			sorter: _leastLoadedSorter,
			in:     []*agent.AgentState{},
			out:    []*agent.AgentState{},
		},

		// sort by number of jobs scheduled to agent
		{
			sorter: _leastLoadedSorter,
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

		// fall back to sorting alphabetically by machine ID when # jobs is equal
		{
			sorter: _leastLoadedSorter,
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

		// sort by sysstat field specified by job
		{
			sorter: _leastSystemStatFieldSorter,
			job: &job.Job{
				Name: "foo.service",
				Unit: newUnitWithSchedulerMetadata(t, "", "sysstatfield=load1"),
			},
			in: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "A",
						Statdata: map[string]float32{"load1": 100.0},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "B",
						Statdata: map[string]float32{"load1": 1.0},
					},
				},
			},
			out: []*agent.AgentState{
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "B",
						Statdata: map[string]float32{"load1": 1.0},
					},
				},
				&agent.AgentState{
					MState: &machine.MachineState{
						ID:       "A",
						Statdata: map[string]float32{"load1": 100.0},
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

		tt.sorter.sortedAgents(sortable, tt.job)
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
