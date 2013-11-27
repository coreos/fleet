package engine

import (
	"errors"
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

		req := s.Registry.ClaimRequest(s.Machine, s.ClaimTTL)
		if req == nil {
			continue
		}

		jobs, err := job.NewJobsFromRequest(req)
		if err != nil {
			log.Printf("Unable to resolve request %s: %s", req.ID, err)
			continue
		}

		machines := s.Registry.GetActiveMachines()

		sched, err := s.BuildSchedule(jobs, machines)
		if err != nil {
			log.Print(err)
			continue
		}

		// For now, we assume that if we can initially acquire the lock
		// we're safe to move forward with scheduling. This is not ideal.
		for j, m := range sched {
			s.Registry.ScheduleJob(&j, m)
		}

		s.Registry.ResolveRequest(req)
	}
}

func (s *Scheduler) BuildSchedule(jobs []job.Job, machines map[string]machine.Machine) (map[job.Job]*machine.Machine, error) {
	sched := map[job.Job]*machine.Machine{}

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

			// Check if the corresponding systemd-service job has been scheduled
			// within this context
			for j2, m := range sched {
				if serviceName == j2.Name {
					mach = m
				}
			}

			if mach == nil {
				service, _ := job.NewJob(serviceName, nil, nil)
				if state := s.Registry.GetJobState(service); state != nil {
					mach = state.Machine
				}
			}

			if mach == nil {
				log.Printf("Unable to schedule job %s since corresponding "+
					"service job %s could not be found", j.Name, serviceName)
			}
		}

		if mach == nil {
			log.Printf("Not scheduling job %s", j.Name)
			return nil
		} else {
			log.Println("Scheduling job", j.Name, "to machine", mach.BootId)
			return mach
		}
	}

	undecided := make([]job.Job, len(jobs))
	copy(undecided, jobs)

	// Iterate over the submitted set of jobs up to N+1 times where N=len(jobs). We assume
	// that N+1 is the theoretical maximum number of attempts that we could possibly take.
	// This is not proven to be true...
	for i := 0; i < len(jobs)+1; i++ {
		decisions := 0

		for i := 0; i < len(undecided); i++ {
			job := undecided[i-decisions]
			mach := decide(&job)
			if mach != nil {
				sched[job] = mach
				undecided = append(undecided[0:i-decisions], undecided[i-decisions+1:]...)
				decisions++
			}
		}
	}

	if len(undecided) > 0 {
		return nil, errors.New("Unable to decide how to schedule all jobs")
	}

	return sched, nil
}

func pickRandomMachine(machines map[string]machine.Machine) *machine.Machine {
	machineSlice := make([]machine.Machine, 0)
	for _, v := range machines {
		machineSlice = append(machineSlice, v)
	}

	target := rand.Intn(len(machineSlice))
	return &machineSlice[target]
}
