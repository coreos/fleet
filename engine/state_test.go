package engine

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/agent"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

func TestClusterStateAgents(t *testing.T) {
	tests := []struct {
		clust  *clusterState
		agents map[string]*agent.AgentState
	}{
		// no data, no agents
		{
			clust: &clusterState{
				jobs:     map[string]*job.Job{},
				machines: map[string]*machine.MachineState{},
			},
			agents: map[string]*agent.AgentState{},
		},

		// job ignored if machine does not exist
		{
			clust: &clusterState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{
						TargetState:     job.JobStateLaunched,
						TargetMachineID: "XXX",
					},
				},
				machines: map[string]*machine.MachineState{},
			},
			agents: map[string]*agent.AgentState{},
		},

		// agentState record exists even without jobs
		{
			clust: &clusterState{
				jobs: map[string]*job.Job{},
				machines: map[string]*machine.MachineState{
					"XXX": &machine.MachineState{ID: "XXX"},
				},
			},
			agents: map[string]*agent.AgentState{
				"XXX": &agent.AgentState{
					MState: &machine.MachineState{ID: "XXX"},
					Units:  map[string]*job.Unit{},
				},
			},
		},

		// only inactive jobs ignored
		{
			clust: &clusterState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateInactive,
						TargetMachineID: "XXX",
					},
					"bar.service": &job.Job{
						Name:            "bar.service",
						TargetState:     job.JobStateLoaded,
						TargetMachineID: "XXX",
					},
					"baz.service": &job.Job{
						Name:            "baz.service",
						TargetState:     job.JobStateLaunched,
						TargetMachineID: "XXX",
					},
				},
				machines: map[string]*machine.MachineState{
					"XXX": &machine.MachineState{ID: "XXX"},
				},
			},
			agents: map[string]*agent.AgentState{
				"XXX": &agent.AgentState{
					MState: &machine.MachineState{ID: "XXX"},
					Units: map[string]*job.Unit{
						"bar.service": &job.Unit{
							Name:        "bar.service",
							TargetState: job.JobStateLoaded,
						},
						"baz.service": &job.Unit{
							Name:        "baz.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
			},
		},

		// multiple jobs, multiple agents
		{
			clust: &clusterState{
				jobs: map[string]*job.Job{
					"foo.service": &job.Job{
						Name:            "foo.service",
						TargetState:     job.JobStateLaunched,
						TargetMachineID: "XXX",
					},
					"bar.service": &job.Job{
						Name:            "bar.service",
						TargetState:     job.JobStateLaunched,
						TargetMachineID: "ZZZ",
					},
					"ping.service": &job.Job{
						Name:            "ping.service",
						TargetState:     job.JobStateLaunched,
						TargetMachineID: "XXX",
					},
					"pong.service": &job.Job{
						Name:            "pong.service",
						TargetState:     job.JobStateLaunched,
						TargetMachineID: "YYY",
					},
				},
				machines: map[string]*machine.MachineState{
					"XXX": &machine.MachineState{ID: "XXX"},
					"YYY": &machine.MachineState{ID: "YYY"},
					"ZZZ": &machine.MachineState{ID: "ZZZ"},
				},
			},
			agents: map[string]*agent.AgentState{
				"XXX": &agent.AgentState{
					MState: &machine.MachineState{ID: "XXX"},
					Units: map[string]*job.Unit{
						"foo.service": &job.Unit{
							Name:        "foo.service",
							TargetState: job.JobStateLaunched,
						},
						"ping.service": &job.Unit{
							Name:        "ping.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
				"YYY": &agent.AgentState{
					MState: &machine.MachineState{ID: "YYY"},
					Units: map[string]*job.Unit{
						"pong.service": &job.Unit{
							Name:        "pong.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
				"ZZZ": &agent.AgentState{
					MState: &machine.MachineState{ID: "ZZZ"},
					Units: map[string]*job.Unit{
						"bar.service": &job.Unit{
							Name:        "bar.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		agents := tt.clust.agents()
		if !reflect.DeepEqual(tt.agents, agents) {
			t.Errorf("case %d: incorrect agents", i)
		}
	}
}
