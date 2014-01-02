package job

import (
	"testing"

	"github.com/coreos/coreinit/machine"
)

func TestNewJobNilStateNilPayloadNoRequirements(t *testing.T) {
	j1, _ := NewJob("ping.service", nil, nil, make(map[string][]string, 0))

	if j1.Name != "ping.service" {
		t.Fatal("job.Job.Name != 'ping.service'")
	}

	if j1.Type != "systemd-service" {
		t.Fatal("job.Job.Name != 'systemd-service'")
	}

	if j1.State != nil {
		t.Fatal("job.Job.State != nil")
	}

	if j1.Payload != nil {
		t.Fatal("job.Job.Payload != nil")
	}

	if len(j1.Requirements) != 0 {
		t.Fatal("job.Job.Requirements are incorrect")
	}
}

func TestNewJob(t *testing.T) {
	mach := machine.New("XXX", "", make(map[string]string, 0))
	js1 := NewJobState("loaded", "inactive", "running", []string{}, mach)
	jp1 := &JobPayload{"echo.service", "Echo"}

	j1, _ := NewJob("pong.service", js1, jp1, map[string][]string{"foo": []string{"bar"}})

	if j1.Name != "pong.service" {
		t.Fatal("job.Job.Name != 'pong.service'")
	}

	if j1.Type != "systemd-service" {
		t.Fatal("job.Job.Name != 'systemd-service'")
	}

	if j1.State != js1 {
		t.Fatal("job.Job.State does not match expected value")
	}

	if j1.Payload != jp1 {
		t.Fatal("job.Job.Payload does not match expected value")
	}

	if len(j1.Requirements) != 1 || len(j1.Requirements["foo"]) != 1 || j1.Requirements["foo"][0] != "bar" {
		t.Fatal("job.Job.Requirements are incorrect")
	}

}

func TestNewJobBadType(t *testing.T) {
	j, err := NewJob("bad-type", nil, nil, make(map[string][]string, 0))

	if err == nil {
		t.Fatal("Expected non-nil error")
	}

	if j != nil {
		t.Fatal("Expected nil response")
	}
}
