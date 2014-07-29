package engine

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
)

func TestClusterStateJobs(t *testing.T) {
	jobs := []job.Job{
		job.Job{Name: "1.service", TargetState: job.JobStateInactive, TargetMachineID: ""},
		job.Job{Name: "2.service", TargetState: job.JobStateLoaded, TargetMachineID: ""},
		job.Job{Name: "3.service", TargetState: job.JobStateLaunched, TargetMachineID: ""},
		job.Job{Name: "4.service", TargetState: job.JobStateLoaded, TargetMachineID: "XXX"},
		job.Job{Name: "5.service", TargetState: job.JobStateLaunched, TargetMachineID: "YYY"},
	}
	cs := newClusterState(jobs, map[string]pkg.Set{}, []machine.MachineState{})

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

func TestClusterStateOfferExists(t *testing.T) {
	offers := map[string]pkg.Set{
		"foo.service": pkg.NewUnsafeSet(),
		"bar.service": pkg.NewUnsafeSet(),
	}
	cs := newClusterState([]job.Job{}, offers, []machine.MachineState{})

	expect := map[string]pkg.Set{
		"foo.service": pkg.NewUnsafeSet(),
		"bar.service": pkg.NewUnsafeSet(),
	}
	if !reflect.DeepEqual(expect, cs.offers) {
		t.Fatalf("Expected %v, got %v", expect, cs.offers)
	}

	if !cs.offerExists("foo.service") {
		t.Fatalf("Offer for foo.service does not exist")
	}

	if !cs.offerExists("bar.service") {
		t.Fatalf("Offer for bar.service does not exist")
	}

	if cs.offerExists("not-found") {
		t.Fatalf("Offer for not-found exists")
	}

	cs.forgetOffer("foo.service")

	expect = map[string]pkg.Set{
		"bar.service": pkg.NewUnsafeSet(),
	}
	if !reflect.DeepEqual(expect, cs.offers) {
		t.Fatalf("Expected %v, got %v", expect, cs.offers)
	}

	if cs.offerExists("foo.service") {
		t.Fatalf("Offer for foo.service still exists")
	}

	if !cs.offerExists("bar.service") {
		t.Fatalf("Offer for bar.service does not exist")
	}

	if cs.offerExists("not-found") {
		t.Fatalf("Offer for not-found exists")
	}
}

func TestClusterStateMachineExists(t *testing.T) {
	machines := []machine.MachineState{
		machine.MachineState{ID: "XXX"},
	}
	cs := newClusterState([]job.Job{}, map[string]pkg.Set{}, machines)

	if !cs.machineExists("XXX") {
		t.Fatalf("Machine XXX does not exist")
	}

	if cs.machineExists("YYY") {
		t.Fatalf("Machine YYY exists")
	}
}
