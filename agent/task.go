// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"fmt"

	"github.com/coreos/fleet/job"
)

const (
	taskTypeLoadUnit        = "LoadUnit"
	taskTypeUnloadUnit      = "UnloadUnit"
	taskTypeStartUnit       = "StartUnit"
	taskTypeStopUnit        = "StopUnit"
	taskTypeReloadUnitFiles = "ReloadUnitFiles"

	taskReasonScheduledButNotRunnable    = "unit scheduled locally but unable to run"
	taskReasonScheduledButUnloaded       = "unit scheduled here but not loaded"
	taskReasonLoadedButNotScheduled      = "unit loaded but not scheduled here"
	taskReasonLoadedButHashDiffers       = "unit loaded but hash differs to expected"
	taskReasonLoadedDesiredStateLaunched = "unit currently loaded but desired state is launched"
	taskReasonLaunchedDesiredStateLoaded = "unit currently launched but desired state is loaded"
	taskReasonPurgingAgent               = "purging agent"
	taskReasonAlwaysReloadUnitFiles      = "always reload unit files"
)

type task struct {
	typ    string
	reason string
	unit   *job.Unit
}

var taskTypeSortOrder = map[string]int{
	taskTypeUnloadUnit:      1,
	taskTypeLoadUnit:        2,
	taskTypeReloadUnitFiles: 3,
	taskTypeStopUnit:        4,
	taskTypeStartUnit:       5,
}

type sortableTasks []task

func (st sortableTasks) Len() int      { return len(st) }
func (st sortableTasks) Swap(i, j int) { st[i], st[j] = st[j], st[i] }
func (st sortableTasks) Less(i, j int) bool {
	return taskTypeSortOrder[st[i].typ] < taskTypeSortOrder[st[j].typ]
}

type taskResult struct {
	task task
	err  error
}

type taskManager struct {
	mapper taskMapperFunc
}

func newTaskManager() *taskManager {
	return &taskManager{
		mapper: mapTaskToFunc,
	}
}

// Do attempts to complete a series of tasks against an Agent. Each task
// is executed in order. If any task is unable to be attempted, or is able
// to be attempted but fails, Do will halt execution. The returned slice
// will contain a taskResult for every task that was attempted. Do is not
// threadsafe.
func (tm *taskManager) Do(tasks []task, a *Agent) []taskResult {
	results := make([]taskResult, 0, len(tasks))
	for _, t := range tasks {
		taskFunc, err := tm.mapper(t, a)
		if err == nil {
			err = taskFunc()
		}

		results = append(results, taskResult{task: t, err: err})

		if err != nil {
			break
		}
	}

	return results
}

type taskMapperFunc func(t task, a *Agent) (func() error, error)

func mapTaskToFunc(t task, a *Agent) (fn func() error, err error) {
	switch t.typ {
	case taskTypeLoadUnit:
		fn = func() error { return a.loadUnit(t.unit) }
	case taskTypeUnloadUnit:
		fn = func() error { a.unloadUnit(t.unit.Name); return nil }
	case taskTypeStartUnit:
		fn = func() error { a.startUnit(t.unit.Name); return nil }
	case taskTypeStopUnit:
		fn = func() error { a.stopUnit(t.unit.Name); return nil }
	case taskTypeReloadUnitFiles:
		fn = func() error { return a.reloadUnitFiles() }
	default:
		err = fmt.Errorf("unrecognized task type %q", t.typ)
	}

	return
}
