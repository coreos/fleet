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

type taskChain struct {
	job   *job.Job
	tasks []task
}

func newTaskChain(j *job.Job, t ...task) taskChain {
	return taskChain{
		job:   j,
		tasks: t,
	}
}

func (tc *taskChain) Add(t task) {
	tc.tasks = append(tc.tasks, t)
}

type task struct {
	typ    string
	reason string
}

type taskResult struct {
	task task
	err  error
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
func (tm *taskManager) Do(tc taskChain, a *Agent) (chan taskResult, error) {
	if tc.job == nil {
		return nil, errors.New("unable to handle task with nil Job")
	}

	if tm.processing.Contains(tc.job.Name) {
		return nil, errors.New("task already in flight")
	}

	// Do is not threadsafe due to the race between Contains and Add
	tm.processing.Add(tc.job.Name)

	reschan := make(chan taskResult, len(tc.tasks))
	go func() {
		defer tm.processing.Remove(tc.job.Name)
		for _, t := range tc.tasks {
			t := t
			res := taskResult{
				task: t,
			}

			taskFunc, err := tm.mapper(t, tc.job, a)
			if err != nil {
				res.err = err
			} else {
				res.err = taskFunc()
			}

			reschan <- res
		}

		close(reschan)
	}()

	return reschan, nil
}

type taskMapperFunc func(t task, j *job.Job, a *Agent) (func() error, error)

func mapTaskToFunc(t task, j *job.Job, a *Agent) (fn func() error, err error) {
	switch t.typ {
	case taskTypeLoadJob:
		fn = func() error { return a.loadJob(j) }
	case taskTypeUnloadJob:
		fn = func() error { a.unloadJob(j.Name); return nil }
	case taskTypeStartJob:
		fn = func() error { a.startJob(j.Name); return nil }
	case taskTypeStopJob:
		fn = func() error { a.stopJob(j.Name); return nil }
	default:
		err = fmt.Errorf("unrecognized task type %q", t.typ)
	}

	return
}
