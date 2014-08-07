package agent

import (
	"testing"
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/pkg"
)

func TestTaskManagerTwoInFlight(t *testing.T) {
	result := make(chan error)
	testMapper := func(*task, *Agent) (exec func() error, err error) {
		exec = func() error {
			return <-result
		}
		return
	}

	tm := taskManager{
		processing: pkg.NewUnsafeSet(),
		mapper:     testMapper,
	}

	errchan1, err := tm.Do(&task{Job: &job.Job{Name: "foo"}}, nil)
	if err != nil {
		t.Fatalf("unable to start task: %v", err)
	}

	errchan2, err := tm.Do(&task{Job: &job.Job{Name: "bar"}}, nil)
	if err != nil {
		t.Fatalf("unable to start task: %v", err)
	}

	close(result)

	go func() {
		<-time.After(time.Second)
		t.Fatalf("expected errchans to be ready to receive within 1s")
	}()

	err = <-errchan1
	if err != nil {
		t.Fatalf("received unexpected error from task one: %v", err)
	}

	err = <-errchan2
	if err != nil {
		t.Fatalf("received unexpected error from task two: %v", err)
	}
}

func TestTaskManagerJobSerialization(t *testing.T) {
	result := make(chan error)
	testMapper := func(*task, *Agent) (exec func() error, err error) {
		exec = func() error {
			return <-result
		}
		return
	}

	tm := taskManager{
		processing: pkg.NewUnsafeSet(),
		mapper:     testMapper,
	}

	errchan, err := tm.Do(&task{Job: &job.Job{Name: "foo"}}, nil)
	if err != nil {
		t.Fatalf("unable to start first task: %v", err)
	}

	// the first task should block the second, as it is still in progress
	_, err = tm.Do(&task{Job: &job.Job{Name: "foo"}}, nil)
	if err == nil {
		t.Fatalf("expected error from attempt to start second task, got nil")
	}

	result <- nil

	select {
	case err := <-errchan:
		if err != nil {
			t.Errorf("received unexpected error from first task: %v", err)
		}
	default:
		t.Errorf("expected errchan to be ready to receive")
	}

	// since the first task completed, this third task can start
	_, err = tm.Do(&task{Job: &job.Job{Name: "foo"}}, nil)
	if err != nil {
		t.Fatalf("unable to start third task: %v", err)
	}

	close(result)
}

func TestTaskmanagerTaskValidity(t *testing.T) {
	tests := []struct {
		task *task
		ok   bool
	}{
		{
			task: nil,
			ok:   false,
		},

		{
			task: &task{Job: nil},
			ok:   false,
		},

		{
			task: &task{Job: &job.Job{Name: "foo"}},
			ok:   true,
		},
	}

	successfulTaskMapper := func(t *task, a *Agent) (func() error, error) {
		return func() error { return nil }, nil
	}

	tm := taskManager{
		processing: pkg.NewUnsafeSet(),
		mapper:     successfulTaskMapper,
	}

	for i, tt := range tests {
		_, err := tm.Do(tt.task, nil)
		if tt.ok != (err == nil) {
			t.Errorf("case %d: expected ok=%t, got err=%v", i, tt.ok, err)
		}
	}
}
