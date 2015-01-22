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
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
)

const (
	// time between triggering reconciliation routine
	reconcileInterval = 5 * time.Second
)

func NewReconciler(reg registry.Registry, rStream pkg.EventStream) *AgentReconciler {
	return &AgentReconciler{
		reg:      reg,
		rStream:  rStream,
		tManager: newTaskManager(),
	}
}

type AgentReconciler struct {
	reg      registry.Registry
	rStream  pkg.EventStream
	tManager *taskManager
}

// Run periodically attempts to reconcile the provided Agent until the stop
// channel is closed. Run will also reconcile in reaction to events on the
// AgentReconciler's rStream.
func (ar *AgentReconciler) Run(a *Agent, stop chan bool) {
	reconcile := func() {
		start := time.Now()
		ar.Reconcile(a)
		elapsed := time.Now().Sub(start)

		msg := fmt.Sprintf("AgentReconciler completed reconciliation in %s", elapsed)
		if elapsed > reconcileInterval {
			log.Warning(msg)
		} else {
			log.Debug(msg)
		}
	}
	reconciler := pkg.NewPeriodicReconciler(reconcileInterval, reconcile, ar.rStream)
	reconciler.Run(stop)
}

// Reconcile drives the local Agent's state towards the desired state
// stored in the Registry.
func (ar *AgentReconciler) Reconcile(a *Agent) {
	dAgentState, err := desiredAgentState(a, ar.reg)
	if err != nil {
		log.Errorf("Unable to determine agent's desired state: %v", err)
		return
	}

	cAgentState, err := a.units()
	if err != nil {
		log.Errorf("Unable to determine agent's current state: %v", err)
		return
	}

	for tc := range ar.calculateTaskChainsForUnits(dAgentState, cAgentState) {
		ar.launchTaskChain(tc, a)
	}
}

// Purge attempts to unload all Units that have been loaded locally
func (ar *AgentReconciler) Purge(a *Agent) {
	for {
		cAgentState, err := a.units()
		if err != nil {
			log.Errorf("Unable to determine agent's current state: %v", err)
			return
		}
		if len(cAgentState) == 0 {
			return
		}

		for name, _ := range cAgentState {
			t := task{
				typ:    taskTypeUnloadUnit,
				reason: taskReasonPurgingAgent,
			}
			u := &job.Unit{
				Name: name,
			}
			tc := newTaskChain(u, t)
			ar.launchTaskChain(tc, a)
		}

		time.Sleep(time.Second)
	}
}

// desiredAgentState builds an *AgentState object that represents what the
// provided Agent should currently be doing.
func desiredAgentState(a *Agent, reg registry.Registry) (*AgentState, error) {
	units, err := reg.Units()
	if err != nil {
		log.Errorf("Failed fetching Units from Registry: %v", err)
		return nil, err
	}

	sUnits, err := reg.Schedule()
	if err != nil {
		log.Errorf("Failed fetching schedule from Registry: %v", err)
		return nil, err
	}

	ms := a.Machine.State()
	as := AgentState{
		MState: &ms,
		Units:  make(map[string]*job.Unit),
	}

	sUnitMap := make(map[string]*job.ScheduledUnit)
	for _, sUnit := range sUnits {
		sUnit := sUnit
		sUnitMap[sUnit.Name] = &sUnit
	}

	for _, u := range units {
		u := u
		md := u.RequiredTargetMetadata()
		if u.IsGlobal() && !machine.HasMetadata(&ms, md) {
			log.Debugf("Agent unable to run global unit %s: missing required metadata", u.Name)
			continue
		}
		if !u.IsGlobal() {
			sUnit, ok := sUnitMap[u.Name]
			if !ok || sUnit.TargetMachineID == "" || sUnit.TargetMachineID != ms.ID {
				continue
			}
		}
		as.Units[u.Name] = &u
	}

	return &as, nil
}

// calculateTaskChainsForUnits compares the desired and current state of an Agent.
// The generated taskChains represent what should be done to make the desired
// state match the current state.
func (ar *AgentReconciler) calculateTaskChainsForUnits(dState *AgentState, cState unitStates) <-chan taskChain {
	tcChan := make(chan taskChain)
	go func() {
		jobs := pkg.NewUnsafeSet()
		for cName := range cState {
			jobs.Add(cName)
		}

		for dName := range dState.Units {
			jobs.Add(dName)
		}

		for _, name := range jobs.Values() {
			tc := ar.calculateTaskChainForUnit(dState, cState, name)
			if tc == nil {
				continue
			}
			tcChan <- *tc
		}

		close(tcChan)
	}()

	return tcChan
}

func (ar *AgentReconciler) calculateTaskChainForUnit(dState *AgentState, cState unitStates, jName string) *taskChain {
	var dJob *job.Unit
	var dJHash string
	if dState != nil {
		dJob = dState.Units[jName]
		if dJob != nil {
			dJHash = dJob.Unit.Hash().String()
		}
	}
	var cJState *job.JobState
	var cJHash string
	if us, ok := cState[jName]; ok {
		cJState = &us.state
		cJHash = us.hash
	}
	if dJob == nil && cJState == nil {
		log.Errorf("Desired state and current state of Job(%s) nil, not sure what to do", jName)
		return nil
	}

	if dJob == nil || dJob.TargetState == job.JobStateInactive {
		if cJState == nil {
			return nil
		}
		t := task{
			typ:    taskTypeUnloadUnit,
			reason: taskReasonLoadedButNotScheduled,
		}
		u := &job.Unit{
			Name: jName,
		}
		tc := newTaskChain(u, t)
		return &tc
	}

	u := &job.Unit{
		Name: jName,
		Unit: dJob.Unit,
	}

	if cJState == nil {
		tc := newTaskChain(u)
		tc.Add(task{
			typ:    taskTypeLoadUnit,
			reason: taskReasonScheduledButUnloaded,
		})

		// as an optimization, queue the unit for launching immediately after loading
		if dJob.TargetState == job.JobStateLaunched {
			tc.Add(task{
				typ:    taskTypeStartUnit,
				reason: taskReasonLoadedDesiredStateLaunched,
			})
		}

		return &tc
	}

	if cJHash != dJHash {
		log.Debugf("Desired hash %q differs to current hash %s of Job(%s) - unloading", dJHash, cJHash, jName)
		tc := newTaskChain(u)
		tc.Add(task{
			typ:    taskTypeUnloadUnit,
			reason: taskReasonLoadedButHashDiffers,
		})

		// queue the correct unit for loading immediately after unloading the old one
		tc.Add(task{
			typ:    taskTypeLoadUnit,
			reason: taskReasonScheduledButUnloaded,
		})

		// as an optimization, queue the unit for launching immediately after loading
		if dJob.TargetState == job.JobStateLaunched {
			tc.Add(task{
				typ:    taskTypeStartUnit,
				reason: taskReasonLoadedDesiredStateLaunched,
			})
		}

		return &tc
	}

	if *cJState == dJob.TargetState {
		log.Debugf("Desired state %q matches current state of Job(%s), nothing to do", *cJState, jName)
		return nil
	}

	tc := newTaskChain(u)
	if *cJState == job.JobStateInactive {
		tc.Add(task{
			typ:    taskTypeLoadUnit,
			reason: taskReasonScheduledButUnloaded,
		})
	}

	if (*cJState == job.JobStateInactive || *cJState == job.JobStateLoaded) && dJob.TargetState == job.JobStateLaunched {
		tc.Add(task{
			typ:    taskTypeStartUnit,
			reason: taskReasonLoadedDesiredStateLaunched,
		})
	}

	if *cJState == job.JobStateLaunched && dJob.TargetState == job.JobStateLoaded {
		tc.Add(task{
			typ:    taskTypeStopUnit,
			reason: taskReasonLaunchedDesiredStateLoaded,
		})
	}

	if len(tc.tasks) == 0 {
		log.Errorf("Unable to determine how to reconcile Job(%s): desiredState=%#v currentState=%#v", jName, dJob, cJState)
		return nil
	}

	return &tc
}

func (ar *AgentReconciler) launchTaskChain(tc taskChain, a *Agent) {
	log.Debugf("AgentReconciler attempting task chain %s", tc)
	reschan, err := ar.tManager.Do(tc, a)
	if err != nil {
		log.Infof("AgentReconciler task chain failed: chain=%s err=%v", tc, err)
		return
	}

	go func() {
		for res := range reschan {
			if res.err == nil {
				log.Infof("AgentReconciler completed task: type=%s job=%s reason=%q", res.task.typ, tc.unit.Name, res.task.reason)
			} else {
				log.Infof("AgentReconciler task failed: type=%s job=%s reason=%q err=%v", res.task.typ, tc.unit.Name, res.task.reason, res.err)
			}
		}
	}()
}
