package engine

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

func TestClusterStateJobs(t *testing.T) {
	jobs := []job.Job{
		job.Job{Name: "1.service", TargetState: job.JobStateInactive, TargetMachineID: ""},
		job.Job{Name: "2.service", TargetState: job.JobStateLoaded, TargetMachineID: ""},
		job.Job{Name: "3.service", TargetState: job.JobStateLaunched, TargetMachineID: ""},
		job.Job{Name: "4.service", TargetState: job.JobStateLoaded, TargetMachineID: "XXX"},
		job.Job{Name: "5.service", TargetState: job.JobStateLaunched, TargetMachineID: "YYY"},
	}
	cs := newClusterState(jobs, []machine.MachineState{})

	actual := cs.inactiveJobs()
	expect := []*job.Job{
		&job.Job{Name: "1.service", TargetState: job.JobStateInactive, TargetMachineID: ""},
	}
	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("Expected inactiveJobs() = %v, got %v", expect, actual)
	}

	actual = cs.unscheduledLoadedJobs()
	expect = []*job.Job{
		&job.Job{Name: "2.service", TargetState: job.JobStateLoaded, TargetMachineID: ""},
		&job.Job{Name: "3.service", TargetState: job.JobStateLaunched, TargetMachineID: ""},
	}
	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("Expected unscheduledLoadedJobs() = %v, got %v", expect, actual)
	}

	actual = cs.scheduledLoadedJobs()
	expect = []*job.Job{
		&job.Job{Name: "4.service", TargetState: job.JobStateLoaded, TargetMachineID: "XXX"},
		&job.Job{Name: "5.service", TargetState: job.JobStateLaunched, TargetMachineID: "YYY"},
	}
	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("Expected scheduledLoadedJobs() = %v, got %v", expect, actual)
	}

}

func TestClusterStateMachineExists(t *testing.T) {
	machines := []machine.MachineState{
		machine.MachineState{ID: "XXX"},
	}
	cs := newClusterState([]job.Job{}, machines)

	if !cs.machineExists("XXX") {
		t.Fatalf("Machine XXX does not exist")
	}

	if cs.machineExists("YYY") {
		t.Fatalf("Machine YYY exists")
	}
}
