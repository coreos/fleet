package engine

import (
	"errors"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/mutex"
	"github.com/coreos/fleet/registry"
)

type Engine struct {
	registry *registry.Registry
	events   *event.EventBus
	machine  *machine.Machine
	// keeps a picture of the load in the cluster for more intelligent scheduling
	clust *cluster
	stop  chan bool
}

func New(reg *registry.Registry, events *event.EventBus, mach *machine.Machine) *Engine {
	return &Engine{reg, events, mach, newCluster(), nil}
}

func (self *Engine) Run() {
	self.stop = make(chan bool)

	handler := NewEventHandler(self)
	self.events.AddListener("engine", self.machine, handler)

	// Block until we receive a stop signal
	<-self.stop

	self.events.RemoveListener("engine", self.machine)
}

func (self *Engine) Stop() {
	log.V(1).Info("Stopping Engine")
	close(self.stop)
}

func (self *Engine) GetJobsScheduledToMachine(machBootId string) []job.Job {
	var jobs []job.Job

	for _, j := range self.registry.GetAllJobs() {
		tgt := self.registry.GetJobTarget(j.Name)
		if tgt == nil || tgt.BootId != machBootId {
			continue
		}
		jobs = append(jobs, j)
	}

	return jobs
}

func (self *Engine) UnscheduleJob(jobName string) {
	self.registry.UnscheduleJob(jobName)
	log.Infof("Unscheduled Job(%s)", jobName)
}

func (self *Engine) OfferJob(j job.Job) error {
	log.V(2).Infof("Attempting to lock Job(%s)", j.Name)

	mutex := self.lockJob(j.Name)
	if mutex == nil {
		log.V(1).Infof("Could not lock Job(%s)", j.Name)
		return errors.New("Could not lock Job")
	}
	defer mutex.Unlock()

	log.V(1).Infof("Claimed Job", j.Name)

	machineBootIds, err := self.partitionCluster(&j)
	if err != nil {
		log.Errorf("Failed partitioning cluster for Job(%s): %v", j.Name, err)
		return err
	}

	offer := job.NewOfferFromJob(j, machineBootIds)

	log.V(2).Infof("Publishing JobOffer(%s)", offer.Job.Name)
	self.registry.CreateJobOffer(offer)
	log.Infof("Published JobOffer(%s)", offer.Job.Name)

	return nil
}

func (self *Engine) ResolveJobOffer(jobName string, machBootId string) error {
	log.V(2).Infof("Attempting to lock JobOffer(%s)", jobName)
	mutex := self.lockJobOffer(jobName)

	if mutex == nil {
		log.V(2).Infof("Could not lock JobOffer(%s)", jobName)
		return errors.New("Could not lock JobOffer")
	}
	defer mutex.Unlock()

	log.V(2).Infof("Claimed JobOffer(%s)", jobName)

	log.V(2).Infof("Resolving JobOffer(%s), scheduling to Machine(%s)", jobName, machBootId)
	err := self.registry.ResolveJobOffer(jobName)
	if err != nil {
		log.Errorf("Failed resolving JobOffer(%s): %v", jobName, err)
		return err
	}

	err = self.registry.ScheduleJob(jobName, machBootId)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s): %v", jobName, err)
		return err
	}

	log.Infof("Scheduled Job(%s) to Machine(%s)", jobName, machBootId)
	return nil
}

func (self *Engine) RemoveJobState(jobName string) {
	self.registry.RemoveJobState(jobName)
}

func (self *Engine) lockJobOffer(jobName string) *mutex.TimedResourceMutex {
	return self.registry.LockJobOffer(jobName, self.machine.State().BootId)
}

func (self *Engine) lockJob(jobName string) *mutex.TimedResourceMutex {
	return self.registry.LockJob(jobName, self.machine.State().BootId)
}

// Pass-through to Registry.LockMachine
func (self *Engine) LockMachine(machBootId string) *mutex.TimedResourceMutex {
	return self.registry.LockMachine(machBootId, self.machine.State().BootId)
}
