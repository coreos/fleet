/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package agent

import (
	"testing"
	"time"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/pkg"
)

func TestTaskManagerTwoInFlight(t *testing.T) {
	result := make(chan error)
	testMapper := func(task, *job.Unit, *Agent) (exec func() error, err error) {
		exec = func() error {
			return <-result
		}
		return
	}

	tm := taskManager{
		processing: pkg.NewUnsafeSet(),
		mapper:     testMapper,
	}

	errchan1, err := tm.Do(taskChain{unit: &job.Unit{Name: "foo"}, tasks: []task{task{typ: "test"}}}, nil)
	if err != nil {
		t.Fatalf("unable to start task: %v", err)
	}

	errchan2, err := tm.Do(taskChain{unit: &job.Unit{Name: "bar"}, tasks: []task{task{typ: "test"}}}, nil)
	if err != nil {
		t.Fatalf("unable to start task: %v", err)
	}

	close(result)

	go func() {
		<-time.After(time.Second)
		t.Fatalf("expected errchans to be ready to receive within 1s")
	}()

	res := <-errchan1
	if res.err != nil {
		t.Fatalf("received unexpected error from task one: %v", res.err)
	}

	res = <-errchan2
	if res.err != nil {
		t.Fatalf("received unexpected error from task two: %v", res.err)
	}
}

func TestTaskManagerUnitSerialization(t *testing.T) {
	result := make(chan error)
	testMapper := func(task, *job.Unit, *Agent) (exec func() error, err error) {
		exec = func() error {
			return <-result
		}
		return
	}

	tm := taskManager{
		processing: pkg.NewUnsafeSet(),
		mapper:     testMapper,
	}

	reschan, err := tm.Do(taskChain{unit: &job.Unit{Name: "foo"}, tasks: []task{task{typ: "test"}}}, nil)
	if err != nil {
		t.Fatalf("unable to start first task: %v", err)
	}

	// the first task should block the second, as it is still in progress
	_, err = tm.Do(taskChain{unit: &job.Unit{Name: "foo"}, tasks: []task{task{typ: "test"}}}, nil)
	if err == nil {
		t.Fatalf("expected error from attempt to start second task, got nil")
	}

	result <- nil

	select {
	case res := <-reschan:
		if res.err != nil {
			t.Errorf("received unexpected error from first task: %v", err)
		}
	default:
		t.Errorf("expected reschan to be ready to receive")
	}

	// since the first task completed, this third task can start
	_, err = tm.Do(taskChain{unit: &job.Unit{Name: "foo"}, tasks: []task{task{typ: "test"}}}, nil)
	if err != nil {
		t.Fatalf("unable to start third task: %v", err)
	}

	close(result)
}
