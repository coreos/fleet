package engine

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

type Scheduler struct {
}

func NewScheduler() *Scheduler {
	return &Scheduler{}
}

func (scheduler *Scheduler) BuildSchedule(jobs []job.Job, machines map[string]machine.Machine, reg *registry.Registry) (Schedule, error) {
	schedule := NewScheduleFromJobs(jobs)
	err := scheduler.FinalizeSchedule(&schedule, machines, reg)
	return schedule, err
}

func (scheduler *Scheduler) FinalizeSchedule(schedule *Schedule, machines map[string]machine.Machine, reg *registry.Registry) error {
	decide := func(j *job.Job) *machine.Machine {
		var mach *machine.Machine
		// If the Job being scheduled is a systemd service unit, we assume we
		// can put it anywhere. If not, we must find the machine where the
		// Job's related service file is currently scheduled.
		if j.Type == "systemd-service" {
			mach = pickRandomMachine(machines)
		} else {
			// This is intended to match a standard filetype (i.e. '.socket' in 'web.socket')
			re := regexp.MustCompile("\\.(.[a-z]*)$")
			serviceName := re.ReplaceAllString(j.Name, ".service")

			// Check if the corresponding systemd-service job is referenced in the schedule
			// we're actively finalizing
			for j2, m := range *schedule {
				if serviceName == j2.Name {
					mach = m
				}
			}

			if mach == nil {
				service, _ := job.NewJob(serviceName, nil, nil)
				//TODO: Remove registry access from the scheduler
				if state := reg.GetJobState(service); state != nil {
					mach = state.Machine
				}
			}

			if mach == nil {
				log.Infof("Unable to schedule Job(%s) since corresponding "+
					"service Job(%s) could not be found", j.Name, serviceName)
			}
		}

		if mach == nil {
			log.V(1).Infof("Not scheduling Job(%s)", j.Name)
			return nil
		} else {
			log.Infof("Scheduling Job(%s) to Machine(%s)", j.Name, mach.BootId)
			return mach
		}
	}

	var undecided []job.Job
	for j, m := range *schedule {
		// The schedule may come in partially-completed. We assume any previous
		// decisions cannot be changed.
		if m == nil {
			undecided = append(undecided, j)
		}
	}

	// Iterate over the submitted set of undecided jobs up to N+1 times where N=len(jobs).
	// We assume that N+1 is the theoretical maximum number of attempts that we could possibly
	// take. This is not proven to be true...
	iterMax := len(undecided) + 1

	for i := 0; i < iterMax; i++ {
		decisions := 0

		for i := 0; i < len(undecided); i++ {
			job := undecided[i-decisions]
			mach := decide(&job)
			if mach != nil {
				schedule.Add(&job, mach)
				undecided = append(undecided[0:i-decisions], undecided[i-decisions+1:]...)
				decisions++
			}
		}
	}

	if len(undecided) > 0 {
		return errors.New("Unable to decide how to schedule all jobs")
	}

	return nil
}

func pickRandomMachine(machines map[string]machine.Machine) *machine.Machine {
	machineKeySlice := make([]string, len(machines))
	idx := 0
	for k := range machines {
		machineKeySlice[idx] = k
		idx++
	}
	target := machineKeySlice[rand.Intn(len(machineKeySlice))]
	machine := machines[target]
	return &machine
}

type Schedule map[job.Job]*machine.Machine

func NewSchedule() Schedule {
	schedule := make(Schedule, 0)
	return schedule
}

func NewScheduleFromJobs(jobs []job.Job) Schedule {
	schedule := make(Schedule, 0)
	for _, job := range jobs {
		schedule[job] = nil
	}
	return schedule
}

func (self *Schedule) Add(j *job.Job, m *machine.Machine) {
	(*self)[*j] = m
}

func (self *Schedule) MachineJobs(mCheck *machine.Machine) []job.Job {
	jobs := make([]job.Job, 0)
	for j, mSched := range *self {
		if mCheck.BootId == mSched.BootId {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

func (self *Schedule) Unfinished() bool {
	for _, m := range *self {
		if m == nil {
			return true
		}
	}
	return false
}

func (self *Schedule) String() string {
	entries := make([]string, len(*self))
	idx := 0
	for j, m := range *self {
		var bootid string
		if m == nil {
			bootid = "N/A"
		} else {
			bootid = m.BootId
		}
		entries[idx] = fmt.Sprintf("job=%s machine=%s", j.Name, bootid)
		idx++
	}
	return strings.Join(entries, ", ")
}
