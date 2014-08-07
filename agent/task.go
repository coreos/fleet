package agent

import (
	"fmt"

	"github.com/coreos/fleet/job"
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
