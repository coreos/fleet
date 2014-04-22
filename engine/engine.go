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
	eStream  *registry.EventStream
	eBus     *event.EventBus
	machine  *machine.Machine
	// keeps a picture of the load in the cluster for more intelligent scheduling
	clust *cluster
	stop  chan bool
}

func New(reg *registry.Registry, eStream *registry.EventStream, mach *machine.Machine) *Engine {
	eBus := event.NewEventBus()
	e := &Engine{reg, eStream, eBus, mach, newCluster(), nil}

	hdlr := NewEventHandler(e)
	bootID := mach.State().BootID
	eBus.AddListener("engine", bootID, hdlr)

	return e
}

func (self *Engine) Run() {
	self.stop = make(chan bool)

	go self.eBus.Listen(self.stop)
	go self.eStream.Stream(0, self.eBus.Channel, self.stop)
}

func (self *Engine) Stop() {
	log.Info("Stopping Engine")
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

func (self *Engine) OfferJob(j job.Job) error {
	log.V(1).Infof("Attempting to lock Job(%s)", j.Name)

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

	self.registry.CreateJobOffer(offer)
	log.Infof("Published JobOffer(%s)", offer.Job.Name)

	return nil
}

func (self *Engine) ResolveJobOffer(jobName string, machBootID string) error {
	log.V(1).Infof("Attempting to lock JobOffer(%s)", jobName)
	mutex := self.lockJobOffer(jobName)

	if mutex == nil {
		log.V(1).Infof("Could not lock JobOffer(%s)", jobName)
		return errors.New("Could not lock JobOffer")
	}
	defer mutex.Unlock()

	log.V(1).Infof("Claimed JobOffer(%s)", jobName)

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

func (self *Engine) RemoveUnitState(jobName string) {
	self.registry.RemoveUnitState(jobName)
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
