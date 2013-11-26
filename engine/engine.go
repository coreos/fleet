package engine

import (
	"log"
	"math/rand"
	"regexp"
	"time"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

const (
	DefaultClaimTTL = "5s"
)

type Scheduler struct {
	Registry *registry.Registry
	Machine  *machine.Machine
	ClaimTTL time.Duration
}

func NewScheduler(registry *registry.Registry, machine *machine.Machine) *Scheduler {
	claimTTL, _ := time.ParseDuration(DefaultClaimTTL)
	return &Scheduler{registry, machine, claimTTL}
}

func (s *Scheduler) DoSchedule() {
	for true {
		// Let's not be a job-hog
		time.Sleep(time.Second)

		jobs := s.Registry.GetGlobalJobs()
		if len(jobs) == 0 {
			continue
		}

		machines := s.Registry.GetActiveMachines()

		job := s.ClaimJob(jobs)
		if job == nil {
			continue
		}

		// If someone has reported state for this job, we assume
		// it's good to go.
		if jobState := s.Registry.GetJobState(job); jobState != nil {
			continue
		}

		// For now, we assume that if we can initially acquire the lock
		// we're safe to move forward with scheduling. This is not ideal.
		s.ScheduleJob(job, machines)
	}
}

func (s *Scheduler) ClaimJob(jobs map[string]job.Job) *job.Job {
	for _, job := range jobs {
		if s.Registry.AcquireLock(job.Name, s.Machine.BootId, s.ClaimTTL) {
			log.Println("Acquired lock on job", job.Name)
			return &job
		}
	}
	return nil
}

func (s *Scheduler) ScheduleJob(j *job.Job, machines map[string]machine.Machine) {
	var mach *machine.Machine
	// If the Job being scheduled is a systemd service unit, we assume we
	// can put it anywhere. If not, we must find the machine where the
	// Job's related service file is currently scheuled.
	if j.Type == "systemd-service" {
		mach = pickRandomMachine(machines)
	} else {
		// This is intended to match a standard filetype (i.e. '.socket' in 'web.socket')
		re := regexp.MustCompile("\\.(.[a-z]*)$")
		serviceName := re.ReplaceAllString(j.Name, ".service")

		service, _ := job.NewJob(serviceName, nil, nil)
		state := s.Registry.GetJobState(service)

		if state == nil {
			log.Printf("Unable to schedule job %s since corresponding "+
				"service job %s could not be found", j.Name, serviceName)
		} else {
			mach = state.Machine
		}
	}

	if mach == nil {
		log.Printf("Not scheduling job %s", j.Name)
	} else {
		log.Println("Scheduling job", j.Name, "to machine", mach.BootId)
		s.Registry.ScheduleJob(j, mach)
	}
}

func pickRandomMachine(machines map[string]machine.Machine) *machine.Machine {
	machineSlice := make([]machine.Machine, 0)
	for _, v := range machines {
		machineSlice = append(machineSlice, v)
	}

	target := rand.Intn(len(machineSlice))
	return &machineSlice[target]
}
