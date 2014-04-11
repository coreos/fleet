package engine

import (
	"errors"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
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
	bootID := self.machine.State().BootID
	self.events.AddListener("engine", bootID, handler)

	// Block until we receive a stop signal
	<-self.stop

	self.events.RemoveListener("engine", bootID)
}

func (self *Engine) Stop() {
	log.V(1).Info("Stopping Engine")
	close(self.stop)
}

func (self *Engine) GetJobsScheduledToMachine(machBootID string) []job.Job {
	var jobs []job.Job

	for _, j := range self.registry.GetAllJobs() {
		tgt := self.registry.GetJobTarget(j.Name)
		if tgt == "" || tgt != machBootID {
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

	machineBootIDs, err := self.partitionCluster(&j)
	if err != nil {
		log.Errorf("Failed partitioning cluster for Job(%s): %v", j.Name, err)
		return err
	}

	offer := job.NewOfferFromJob(j, machineBootIDs)

	log.V(2).Infof("Publishing JobOffer(%s)", offer.Job.Name)
	self.registry.CreateJobOffer(offer)
	log.Infof("Published JobOffer(%s)", offer.Job.Name)

	return nil
}

func (self *Engine) ResolveJobOffer(jobName string, machBootID string) error {
	log.V(2).Infof("Attempting to lock JobOffer(%s)", jobName)
	mutex := self.lockJobOffer(jobName)

	if mutex == nil {
		log.V(2).Infof("Could not lock JobOffer(%s)", jobName)
		return errors.New("Could not lock JobOffer")
	}
	defer mutex.Unlock()

	log.V(2).Infof("Claimed JobOffer(%s)", jobName)

	log.V(2).Infof("Resolving JobOffer(%s), scheduling to Machine(%s)", jobName, machBootID)
	err := self.registry.ResolveJobOffer(jobName)
	if err != nil {
		log.Errorf("Failed resolving JobOffer(%s): %v", jobName, err)
		return err
	}

	err = self.registry.ScheduleJob(jobName, machBootID)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s): %v", jobName, err)
		return err
	}

	log.Infof("Scheduled Job(%s) to Machine(%s)", jobName, machBootID)
	return nil
}

func (self *Engine) RemovePayloadState(jobName string) {
	self.registry.RemovePayloadState(jobName)
}

func (self *Engine) lockJobOffer(jobName string) *registry.TimedResourceMutex {
	return self.registry.LockJobOffer(jobName, self.machine.State().BootID)
}

func (self *Engine) lockJob(jobName string) *registry.TimedResourceMutex {
	return self.registry.LockJob(jobName, self.machine.State().BootID)
}

// Pass-through to Registry.LockMachine
func (self *Engine) LockMachine(machBootID string) *registry.TimedResourceMutex {
	return self.registry.LockMachine(machBootID, self.machine.State().BootID)
}
