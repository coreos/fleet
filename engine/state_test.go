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

func newUnitWithMetadata(t *testing.T, metadata string) unit.UnitFile {
	contents := fmt.Sprintf("[X-Fleet]\nMachineMetadata=%s", metadata)
	u, err := unit.NewUnitFile(contents)
	if err != nil {
		t.Fatalf("error creating unit from %q: %v", contents, err)
	}
	return *u
}

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

		// ensure basic global jobs are assigned to all agents
		{
			clust: &clusterState{
				gUnits: map[string]*job.Unit{
					"foo.service": &job.Unit{
						Name:        "foo.service",
						TargetState: job.JobStateLoaded,
					},
					"bar.service": &job.Unit{
						Name:        "bar.service",
						TargetState: job.JobStateLaunched,
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
							TargetState: job.JobStateLoaded,
						},
						"bar.service": &job.Unit{
							Name:        "bar.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
				"YYY": &agent.AgentState{
					MState: &machine.MachineState{ID: "YYY"},
					Units: map[string]*job.Unit{
						"foo.service": &job.Unit{
							Name:        "foo.service",
							TargetState: job.JobStateLoaded,
						},
						"bar.service": &job.Unit{
							Name:        "bar.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
				"ZZZ": &agent.AgentState{
					MState: &machine.MachineState{ID: "ZZZ"},
					Units: map[string]*job.Unit{
						"foo.service": &job.Unit{
							Name:        "foo.service",
							TargetState: job.JobStateLoaded,
						},
						"bar.service": &job.Unit{
							Name:        "bar.service",
							TargetState: job.JobStateLaunched,
						},
					},
				},
			},
		},

		// ensure global jobs with metadata are only assigned to appropriate agents
		{
			clust: &clusterState{
				gUnits: map[string]*job.Unit{
					"foo.service": &job.Unit{
						Name:        "foo.service",
						TargetState: job.JobStateLoaded,
						Unit:        newUnitWithMetadata(t, "region=us-west"),
					},
					"bar.service": &job.Unit{
						Name:        "bar.service",
						TargetState: job.JobStateLaunched,
						Unit:        newUnitWithMetadata(t, "disk=ssd"),
					},
				},
				machines: map[string]*machine.MachineState{
					"XXX": &machine.MachineState{
						ID: "XXX",
						Metadata: map[string]string{
							"disk": "ssd",
						},
					},
					"YYY": &machine.MachineState{
						ID: "YYY",
						Metadata: map[string]string{
							"region": "us-west",
						},
					},
					"ZZZ": &machine.MachineState{
						ID: "ZZZ",
						Metadata: map[string]string{
							"foo":    "bar",
							"region": "us-east",
						},
					},
				},
			},
			agents: map[string]*agent.AgentState{
				"XXX": &agent.AgentState{
					MState: &machine.MachineState{
						ID: "XXX",
						Metadata: map[string]string{
							"disk": "ssd",
						},
					},
					Units: map[string]*job.Unit{
						"bar.service": &job.Unit{
							Name:        "bar.service",
							TargetState: job.JobStateLaunched,
							Unit:        newUnitWithMetadata(t, "disk=ssd"),
						},
					},
				},
				"YYY": &agent.AgentState{
					MState: &machine.MachineState{
						ID: "YYY",
						Metadata: map[string]string{
							"region": "us-west",
						},
					},
					Units: map[string]*job.Unit{
						"foo.service": &job.Unit{
							Name:        "foo.service",
							TargetState: job.JobStateLoaded,
							Unit:        newUnitWithMetadata(t, "region=us-west"),
						},
					},
				},
				"ZZZ": &agent.AgentState{
					MState: &machine.MachineState{
						ID: "ZZZ",
						Metadata: map[string]string{
							"foo":    "bar",
							"region": "us-east",
						},
					},
					Units: map[string]*job.Unit{},
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
			msg := fmt.Sprintf("case %d: incorrect agents\n", i)
			msg += "got:\n"
			for id, a := range agents {
				msg += fmt.Sprintf("  %s: %#v\n", id, a)
			}
			msg += "want:\n"
			for id, a := range tt.agents {
				msg += fmt.Sprintf("  %s: %#v\n", id, a)
			}
			t.Error(msg)
		}
	}
}
