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

func (scheduler *Scheduler) DoSchedule() {
	for true {
		// Let's not be a job-hog
		time.Sleep(time.Second)

		request := scheduler.claimRequest()
		if request == nil {
			continue
		}

		if request.IsFlagSet(job.RequestAllMachines) {
			scheduler.persistClusterJobs(request)
		} else {
			machines := scheduler.getMachines()

			schedule, err := NewScheduleFromJobRequest(request, machines)
			if err != nil {
				log.Printf("Unable to resolve job request %s: %s", request.ID, err)
				continue
			}

			err = scheduler.finalizeSchedule(&schedule, machines)
			if err != nil {
				log.Printf("Failed to finalize schedule for job request %s: %s", request.ID, err)
				continue
			}

			err = scheduler.resolveSchedule(request, &schedule)
			if err != nil {
				log.Printf("Failed to resolve schedule for job request %s: %s", request.ID, err)
			}
		}
	}
}

func (scheduler *Scheduler) claimRequest() *job.JobRequest {
	return scheduler.Registry.ClaimRequest(scheduler.Machine, scheduler.ClaimTTL)
}

func (scheduler *Scheduler) getMachines() []machine.Machine {
	machineMap := scheduler.Registry.GetActiveMachines()
	machines := make([]machine.Machine, len(machineMap))
	idx := 0
	for _, m := range machineMap {
		machines[idx] = m
		idx++
	}

	return machines
}

func (scheduler *Scheduler) finalizeSchedule(schedule *Schedule, machines []machine.Machine) error {
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
			// we're actively finalize
			for j2, m := range *schedule {
				if serviceName == j2.Name {
					mach = m
				}
			}

			if mach == nil {
				service, _ := job.NewJob(serviceName, nil, nil)
				if state := scheduler.Registry.GetJobState(service); state != nil {
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
				(*schedule)[job] = mach
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

func pickRandomMachine(machines []machine.Machine) *machine.Machine {
	target := rand.Intn(len(machines))
	return &machines[target]
}

func (scheduler *Scheduler) resolveSchedule(request *job.JobRequest, schedule *Schedule) error {
	// For now, we assume that if we can initially acquire the lock
	// we're safe to move forward with scheduling. This is not ideal.
	for j, m := range *schedule {
		scheduler.Registry.ScheduleMachineJob(&j, m)
	}

	scheduler.Registry.ResolveRequest(request)

	return nil
}

func (scheduler *Scheduler) persistClusterJobs(request *job.JobRequest) {
	for i := 0; i < len(request.Payloads); i++ {
		// Manually create the payload variable so we get a full copy
		// of the data, not just a shallow copy.
		payload := request.Payloads[i]

		//TODO: Handle error from NewJob
		job, _ := job.NewJob(payload.Name, nil, &payload)
		scheduler.Registry.ScheduleClusterJob(job)
	}
	scheduler.Registry.ResolveRequest(request)
}

type Schedule map[job.Job]*machine.Machine

func NewScheduleFromJobRequest(req *job.JobRequest, machines []machine.Machine) (Schedule, error) {
	sched := make(Schedule, 0)

	for i := 0; i < len(req.Payloads); i++ {
		// Manually create the payload variable so we get a full copy
		// of the data, not just a shallow copy.
		payload := req.Payloads[i]

		job, err := job.NewJob(payload.Name, nil, &payload)
		if err != nil {
			return nil, err
		} else {
			sched[*job] = nil
		}
	}
	return sched, nil
}
