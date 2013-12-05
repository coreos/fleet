package engine

import (
	"fmt"
	"time"

	log "github.com/golang/glog"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
	"github.com/coreos/coreinit/registry"
)

const (
	DefaultJobWatchClaimTTL = "10s"
	DefaultRefreshInterval  = "60s"
)

type JobWatcher struct {
	registry        *registry.Registry
	scheduler       *Scheduler
	machine         *machine.Machine
	claimTTL        time.Duration
	refreshInterval time.Duration
	watches         map[string]job.JobWatch
	schedules       map[string]Schedule
	machines        map[string]machine.Machine
}

func NewJobWatcher(reg *registry.Registry, scheduler *Scheduler, m *machine.Machine) *JobWatcher {
	claimTTL, _ := time.ParseDuration(DefaultJobWatchClaimTTL)
	refreshInterval, _ := time.ParseDuration(DefaultRefreshInterval)

	jobs := make(map[string]job.JobWatch, 0)
	schedules := make(map[string]Schedule, 0)
	machines := make(map[string]machine.Machine, 0)

	return &JobWatcher{reg, scheduler, m, claimTTL, refreshInterval, jobs, schedules, machines}
}

func (self *JobWatcher) StartHeartbeatThread() {
	heartbeat := func() {
		for _, watch := range self.watches {
			log.V(1).Infof("Re-claiming JobWatch(%s)", watch.Payload.Name)
			if ok := self.registry.ClaimJobWatch(&watch, self.machine, self.claimTTL); !ok {
				log.V(1).Infof("Failed to re-claim lock on JobWatch(%s)", watch.Payload.Name)
			}
		}
	}

	loop := func() {
		c := time.Tick(self.claimTTL / 2)
		for _ = range c {
			log.V(1).Infof("JobWatcher Heartbeat")
			heartbeat()
		}
	}

	go loop()
}

func (self *JobWatcher) StartRefreshThread() {
	refresh := func() {
		machines := make(map[string]machine.Machine, 0)
		for _, m := range self.registry.GetActiveMachines() {
			machines[m.BootId] = m
		}
		self.machines = machines
	}

	loop := func() {
		for true {
			refresh()
			time.Sleep(self.refreshInterval)
		}
	}

	go loop()
}

func (self *JobWatcher) AddJobWatch(watch *job.JobWatch) bool {
	if !self.registry.ClaimJobWatch(watch, self.machine, self.claimTTL) {
		log.V(1).Infof("Failed to acquire lock on JobWatch(%s)", watch.Payload.Name)
		return false
	}

	log.Infof("Acquired lock on JobWatch(%s), building schedule", watch.Payload.Name)

	self.watches[watch.Payload.Name] = *watch
	sched := NewSchedule()
	self.schedules[watch.Payload.Name] = sched

	if watch.Count == -1 {
		for idx, _ := range self.machines {
			m := self.machines[idx]
			name := fmt.Sprintf("%s.%s", m.BootId, watch.Payload.Name)
			j, _ := job.NewJob(name, nil, watch.Payload)
			log.Infof("Scheduling Job(%s) to Machine(%s)", name, m.BootId)
			sched.Add(j, &m)
		}
	} else {
		for i := 1; i <= watch.Count; i++ {
			name := fmt.Sprintf("%d.%s", i, watch.Payload.Name)
			j, _ := job.NewJob(name, nil, watch.Payload)

			var m *machine.Machine
			// Check if this job was schedule somewhere already
			if state := self.registry.GetJobState(j); state != nil {
				m = state.Machine
			}
			sched.Add(j, m)
		}
	}

	if len(sched) > 0 {
		self.scheduler.FinalizeSchedule(&sched, self.machines, self.registry)
		log.Infof("Submitting schedule: %s", sched.String())
		self.submitSchedule(sched)
	} else {
		log.Infof("No schedule changes made")
	}

	return true
}

func (self *JobWatcher) RemoveJobWatch(watch *job.JobWatch) bool {
	if _, ok := self.watches[watch.Payload.Name]; !ok {
		return false
	}

	delete(self.watches, watch.Payload.Name)

	watchSchedule := self.schedules[watch.Payload.Name]
	delete(self.schedules, watch.Payload.Name)

	for job, mach := range watchSchedule {
		self.registry.RemoveMachineJob(&job, mach)
	}

	return true
}

func (self *JobWatcher) submitSchedule(schedule Schedule) {
	for j, m := range schedule {
		self.registry.ScheduleMachineJob(&j, m)
	}
}

func (self *JobWatcher) TrackMachine(m *machine.Machine) {
	self.machines[m.BootId] = *m

	partial := NewSchedule()
	for _, watch := range self.watches {
		if watch.Count == -1 {
			sched := self.schedules[watch.Payload.Name]

			if len(sched.MachineJobs(m)) > 0 {
				continue
			}

			name := fmt.Sprintf("%s.%s", m.BootId, watch.Payload.Name)
			j, _ := job.NewJob(name, nil, watch.Payload)
			log.Infof("Adding to schedule job=%s machine=%s", name, m.BootId)
			partial.Add(j, m)

			sched.Add(j, m)
		}
	}

	if len(partial) > 0 {
		log.Infof("Submitting schedule: %s", partial.String())
		self.submitSchedule(partial)
	}
}

func (self *JobWatcher) Evacuate(mach *machine.Machine) {
	log.V(1).Infof("Evacuating any scheduled Jobs from Machine(%s)", mach.BootId)
	for _, watch := range self.watches {
		sched := self.schedules[watch.Payload.Name]
		for _, j := range sched.MachineJobs(mach) {
			log.Infof("Removing Job(%s) scheduled to Machine(%s) being evacuated", j.Name, mach.BootId)
			self.registry.RemoveMachineJob(&j, mach)

			if watch.Count != -1 {
				log.Infof("Rescheduling Job(%s) to a new Machine", j.Name)
				sched[j] = nil
			}
		}

		if watch.Count != -1 {
			self.scheduler.FinalizeSchedule(&sched, self.machines, self.registry)
			log.Infof("Schedule changes calculated, submitting")
			self.submitSchedule(sched)
		}
	}
}

func (self *JobWatcher) DropMachine(m *machine.Machine) {
	if _, ok := self.machines[m.BootId]; ok {
		delete(self.machines, m.BootId)
	}
	self.Evacuate(m)
}
