package scheduler

import (
	"log"
	"math/rand"
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

func New(registry *registry.Registry) *Scheduler {
	machine := machine.New("")
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

		machines := s.Registry.GetAllMachines()

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

func (s *Scheduler) ScheduleJob(job *job.Job, machines map[string]machine.Machine) {
	machineSlice := make([]machine.Machine, 0)
	for _, v := range machines {
		machineSlice = append(machineSlice, v)
	}

	target := rand.Intn(len(machineSlice))
	machine := machineSlice[target]

	log.Println("Scheduling job", job.Name, "to machine", machine.BootId)
	s.Registry.ScheduleJob(job, &machine)
}
