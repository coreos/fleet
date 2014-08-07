package agent

import (
	"errors"
	"fmt"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/pkg"
)

const (
	taskTypeLoadJob   = "LoadJob"
	taskTypeUnloadJob = "UnloadJob"
	taskTypeStartJob  = "StartJob"
	taskTypeStopJob   = "StopJob"

	taskReasonScheduledButNotRunnable    = "job scheduled locally but unable to run"
	taskReasonScheduledButUnloaded       = "job scheduled here but not loaded"
	taskReasonLoadedButNotScheduled      = "job loaded but not scheduled here"
	taskReasonLoadedDesiredStateLaunched = "job currently loaded but desired state is launched"
	taskReasonLaunchedDesiredStateLoaded = "job currently launched but desired state is loaded"
	taskReasonPurgingAgent               = "purging agent"
)

type task struct {
	Type   string
	Job    *job.Job
	Reason string
}

func (t *task) String() string {
	var jName string
	if t.Job != nil {
		jName = t.Job.Name
	}
	return fmt.Sprintf("{Type: %s, Job: %s, Reason: %q}", t.Type, jName, t.Reason)
}

type taskManager struct {
	processing pkg.Set
	mapper     taskMapperFunc
}

func newTaskManager() *taskManager {
	return &taskManager{
		processing: pkg.NewUnsafeSet(),
		mapper:     mapTaskToFunc,
	}
}

// Do attempts to complete a task against an Agent. If the task is unable
// to be attempted, an error is returned. A task is unable to be attempted
// if there exists in-flight any task with the same job name. The returned
// error channel will be non-nil only if the task could be attempted. The
// channel will be closed when the task completes. If the task failed, an
// error will be sent to the channel. Do is not threadsafe.
func (tm *taskManager) Do(t *task, a *Agent) (chan error, error) {
	if t == nil {
		return nil, errors.New("unable to handle nil task")
	}

	if t.Job == nil {
		return nil, errors.New("unable to handle task with nil Job")
	}

	if tm.processing.Contains(t.Job.Name) {
		return nil, errors.New("task already in flight")
	}

	taskFunc, err := tm.mapper(t, a)
	if err != nil {
		return nil, err
	}

	tm.processing.Add(t.Job.Name)

	errchan := make(chan error)

	go func() {
		defer tm.processing.Remove(t.Job.Name)
		err := taskFunc()
		if err != nil {
			errchan <- err
		}
		close(errchan)
	}()

	return errchan, nil
}

type taskMapperFunc func(t *task, a *Agent) (func() error, error)

func mapTaskToFunc(t *task, a *Agent) (fn func() error, err error) {
	switch t.Type {
	case taskTypeLoadJob:
		fn = func() error { return a.loadJob(t.Job) }
	case taskTypeUnloadJob:
		fn = func() error { a.unloadJob(t.Job.Name); return nil }
	case taskTypeStartJob:
		fn = func() error { a.startJob(t.Job.Name); return nil }
	case taskTypeStopJob:
		fn = func() error { a.stopJob(t.Job.Name); return nil }
	default:
		err = fmt.Errorf("unrecognized task type %q", t.Type)
	}

	return
}
