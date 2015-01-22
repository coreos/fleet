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
	"encoding/json"

	"github.com/coreos/fleet/job"
)

type agentCache map[string]job.JobState

func (ac *agentCache) MarshalJSON() ([]byte, error) {
	type ds struct {
		TargetStates map[string]job.JobState
	}
	data := ds{
		TargetStates: map[string]job.JobState(*ac),
	}
	return json.Marshal(data)
}

func (ac *agentCache) setTargetState(jobName string, state job.JobState) {
	(*ac)[jobName] = state
}

func (ac *agentCache) dropTargetState(jobName string) {
	delete(*ac, jobName)
}

func (ac *agentCache) launchedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range *ac {
		if ts == job.JobStateLaunched {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (ac *agentCache) loadedJobs() []string {
	jobs := make([]string, 0)
	for j, ts := range *ac {
		if ts == job.JobStateLoaded {
			jobs = append(jobs, j)
		}
	}
	return jobs
}
