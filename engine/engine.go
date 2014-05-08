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
	registry registry.Registry
	eStream  *registry.EventStream
	eBus     *event.EventBus
	machine  *machine.Machine
	// keeps a picture of the load in the cluster for more intelligent scheduling
	clust *cluster
	stop  chan bool
}

func New(reg registry.Registry, eStream *registry.EventStream, mach *machine.Machine) *Engine {
	eBus := event.NewEventBus()
	e := &Engine{reg, eStream, eBus, mach, newCluster(), nil}

	hdlr := NewEventHandler(e)
	eBus.AddListener("engine", hdlr)

	return e
}

func (e *Engine) Run() {
	e.stop = make(chan bool)

	go e.eBus.Listen(e.stop)
	go e.eStream.Stream(0, e.eBus.Channel, e.stop)
}

func (e *Engine) Stop() {
	log.Info("Stopping Engine")
	close(e.stop)
}

func (e *Engine) OfferJob(j job.Job) error {
	log.V(1).Infof("Attempting to lock Job(%s)", j.Name)

	mutex := e.registry.LockJob(j.Name, e.machine.State().ID)
	if mutex == nil {
		log.V(1).Infof("Could not lock Job(%s)", j.Name)
		return errors.New("Could not lock Job")
	}
	defer mutex.Unlock()

	log.V(1).Infof("Claimed Job(%s)", j.Name)

	machineIDs, err := e.partitionCluster(&j)
	if err != nil {
		log.Errorf("Failed partitioning cluster for Job(%s): %v", j.Name, err)
		return err
	}

	offer := job.NewOfferFromJob(j, machineIDs)

	err = e.registry.CreateJobOffer(offer)
	if err == nil {
		log.Infof("Published JobOffer(%s)", offer.Job.Name)
	}

	return err
}

func (e *Engine) ResolveJobOffer(jobName string, machID string) error {
	log.V(1).Infof("Attempting to lock JobOffer(%s)", jobName)
	mutex := e.registry.LockJobOffer(jobName, e.machine.State().ID)

	if mutex == nil {
		log.V(1).Infof("Could not lock JobOffer(%s)", jobName)
		return errors.New("Could not lock JobOffer")
	}
	defer mutex.Unlock()

	log.V(1).Infof("Claimed JobOffer(%s)", jobName)

	err := e.registry.ResolveJobOffer(jobName)
	if err != nil {
		log.Errorf("Failed resolving JobOffer(%s): %v", jobName, err)
		return err
	}

	err = e.registry.ScheduleJob(jobName, machID)
	if err != nil {
		log.Errorf("Failed scheduling Job(%s): %v", jobName, err)
		return err
	}

	log.Infof("Scheduled Job(%s) to Machine(%s)", jobName, machID)
	return nil
}
