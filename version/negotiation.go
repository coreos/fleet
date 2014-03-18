package version

import (
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/mutex"
)

const (
	VersionUninitialized = -1
	NegotiatorTTL = time.Duration(30*time.Second)
)

type Negotiator struct {
	Name		   string
	MinVersion     int
	MaxVersion     int
	CurrentVersion int

	stop chan bool
}

func NewNegotiator(machBootId string, min, max int) (*Negotiator, error) {
	if max < min {
		return nil, fmt.Errorf("MaxVersion must be >= MinVersion, %d < %d", max, min)
	} else if min < 0 {
		return nil, fmt.Errorf("MinVersion must be >= 0, %d <= 0", min)
	}
	return &Negotiator{machBootId, min, max, VersionUninitialized, nil}, nil
}

func (self *Negotiator) GetCurrentVersion() (int, error) {
	if self.MinVersion > self.CurrentVersion || self.MaxVersion < self.CurrentVersion {
		return self.CurrentVersion, fmt.Errorf("Unrecognized cluster version %d", self.CurrentVersion)
	} else {
		return self.CurrentVersion, nil
	}
}

func (self *Negotiator) SetCurrentVersion(version int) error {
	self.CurrentVersion = version
	return nil
}

func (n *Negotiator) Run(cluster ClusterState, events *event.EventBus) {
	n.stop = make(chan bool)

	handler := eventHandler{n, cluster}
	m :=  machine.New(n.Name, "", map[string]string{})
	events.AddListener("negotiator", m, &handler)

	go n.heartbeat(cluster, n.stop)

	// Block until we receive a stop signal
	<-n.stop

	events.RemoveListener("negotiator", m)
}

func (n *Negotiator) heartbeat(cluster ClusterState, stop chan bool) {
	interval := NegotiatorTTL / 2
	for true {
		select {
		case <-stop:
			log.V(2).Info("NegotiatorHeartbeat exiting due to stop signal")
			return
		case <-time.Tick(interval):
			log.V(2).Info("NegotiatorHeartbeat tick")
			err := cluster.Publish(n, NegotiatorTTL)
			if err != nil {
				log.Errorf("NegotiatorHeartbeat failed: %v", err)
			}
		}
	}
}

func (n *Negotiator) Stop() {
	log.V(1).Info("Stopping version negotiation")
	close(n.stop)
}

type ClusterState interface {
	Version() (int, bool, error)
	Upgrade(int) error
	Publish(*Negotiator, time.Duration) error
	Negotiators() ([]Negotiator, error)
	AcquireMutex(*Negotiator) *mutex.TimedResourceMutex
}

type stubClusterState struct {
	mutex       sync.Mutex
	version     int
	negotiators map[string]Negotiator
	events		*event.EventBus
}

func newStubClusterState(eb *event.EventBus) *stubClusterState {
	return &stubClusterState{version: VersionUninitialized, negotiators: make(map[string]Negotiator, 0), events: eb}
}

func (self *stubClusterState) Version() (int, bool, error) {
	if self.version == VersionUninitialized {
		return 0, false, nil
	} else {
		return self.version, true, nil
	}
}

func (self *stubClusterState) Upgrade(version int) error {
	self.version = version

	ev := event.Event{"EventClusterUpgraded", version, nil}
	self.events.Channel<-&ev

	return nil
}

func (self *stubClusterState) Publish(n *Negotiator, ttl time.Duration) error {
	self.negotiators[n.Name] = *n

	ev := event.Event{"EventNegotiatorPublished", *n, nil}
	self.events.Channel<-&ev

	return nil
}

func (self *stubClusterState) Unpublish(n *Negotiator) error {
	delete(self.negotiators, n.Name)
	return nil
}

func (self *stubClusterState) AcquireMutex(n *Negotiator) *mutex.TimedResourceMutex {
	return &mutex.TimedResourceMutex{}
}

func (self stubClusterState) Negotiators() ([]Negotiator, error) {
	negotiators := make([]Negotiator, 0)
	for _, n := range self.negotiators {
		negotiators = append(negotiators, n)
	}
	return negotiators, nil
}

func MaxVersionPossible(negotiators []Negotiator) (max int, err error) {
	max = VersionUninitialized
	length := len(negotiators)
	if length == 0 {
		err = errors.New("Unable to determine max version without any negotiators")
		return
	}
	values := make([]int, length)
	for i, n := range negotiators {
		values[i] = n.MaxVersion
	}

	max = values[0]

	if len(values) == 1 {
		return
	}

	for _, val := range values[1:] {
		if val < max {
			max = val
		}
	}

	return
}
