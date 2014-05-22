package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func TestFakeRegistryJobLifecycle(t *testing.T) {
	reg := NewFakeRegistry(nil, nil, nil, nil)

	jobs, err := reg.GetAllJobs()
	if err != nil {
		t.Fatalf("Received error while calling GetAllJobs: %v", err)
	}
	if !reflect.DeepEqual([]job.Job{}, jobs) {
		t.Fatalf("Expected no jobs, got %v", jobs)
	}

	j1 := job.NewJob("job1.service", *unit.NewUnit(""))
	err = reg.CreateJob(j1)
	if err != nil {
		t.Fatalf("Received error while calling CreateJob: %v", err)
	}

	jobs, err = reg.GetAllJobs()
	if err != nil {
		t.Fatalf("Received error while calling GetAllJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 Job, got %v", jobs)
	}
	if jobs[0].Name != "job1.service" {
		t.Fatalf("Expected Job with name \"job1.service\", got %q", jobs[0].Name)
	}

	err = reg.DestroyJob("job1.service")
	if err != nil {
		t.Fatalf("Received error while calling DestroyJob: %v", err)
	}

	jobs, err = reg.GetAllJobs()
	if err != nil {
		t.Fatalf("Received error while calling GetAllJobs: %v", err)
	}
	if !reflect.DeepEqual([]job.Job{}, jobs) {
		t.Fatalf("Expected no jobs, got %v", jobs)
	}
}
