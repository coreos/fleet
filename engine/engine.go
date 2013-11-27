package engine

import (
	"errors"
	"fmt"
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

		machineMap := s.Registry.GetActiveMachines()
		machines := make([]machine.Machine, len(machineMap))
		idx := 0
		for _, m := range(machineMap) {
			machines[idx] = m
			idx++
		}

		jobs, err := buildScheduleFromRequest(req, machines)
		if err != nil {
			log.Printf("Unable to resolve request %s: %s", req.ID, err)
			continue
		}

		sched, err := s.completeSchedule(jobs, machines)
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

func buildScheduleFromRequest(req *job.JobRequest, machines []machine.Machine) (map[job.Job]*machine.Machine, error) {
	sched := make(map[job.Job]*machine.Machine, 0)

	for i := 0; i < len(req.Payloads); i++ {
		// Manually create the payload variable so we get a full copy
		// of the data, not just a shallow copy.
		payload := req.Payloads[i]

		if req.IsFlagSet(job.RequestAllMachines) {
			log.Printf("Scheduler asked to schedule to all machines")
			for ii := 0; ii < len(machines); ii++ {
				// Manually create the m variable so we get a full copy
				// of the data, not just a shallow copy.
				m := machines[ii]

				//FIXME: This is probably not the correct format for a job name scoped to a given machine.
				jobName := fmt.Sprintf("%s.%s", m.BootId, payload.Name)

				job, err := job.NewJob(jobName, nil, &payload)
				if err != nil {
					return nil, err
				} else {
					sched[*job] = &m
				}
			}
		} else {
			job, err := job.NewJob(payload.Name, nil, &payload)
			if err != nil {
				return nil, err
			} else {
				sched[*job] = nil
			}
		}
	}
	return sched, nil
}

func (s *Scheduler) completeSchedule(sched map[job.Job]*machine.Machine, machines []machine.Machine) (map[job.Job]*machine.Machine, error) {
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

	var undecided []job.Job
	for j, m := range sched {
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

func pickRandomMachine(machines []machine.Machine) *machine.Machine {
	target := rand.Intn(len(machines))
	return &machines[target]
}
