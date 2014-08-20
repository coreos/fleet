package registry

import (
	"path"
	"strings"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
)

const (
	// Occurs when any Job's target is touched
	JobTargetChangeEvent = Event("JobTargetChangeEvent")
	// Occurs when any Job's target state is touched
	JobTargetStateChangeEvent = Event("JobTargetStateChangeEvent")
)

type EventStream interface {
	Next(chan struct{}) chan Event
}

type Event string

type etcdEventStream struct {
	etcd       etcd.Client
	rootPrefix string
}

func NewEventStream(client etcd.Client, rootPrefix string) (EventStream, error) {
	return &etcdEventStream{client, rootPrefix}, nil
}

// Next returns a channel which will emit an Event as soon as one of interest occurs
func (es *etcdEventStream) Next(stop chan struct{}) chan Event {
	evchan := make(chan Event)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			res := watch(es.etcd, path.Join(es.rootPrefix, jobPrefix), stop)
			if ev, ok := parse(res, es.rootPrefix); ok {
				evchan <- ev
				return
			}
		}

	}()

	return evchan
}

func parse(res *etcd.Result, prefix string) (ev Event, ok bool) {
	if res == nil || res.Node == nil {
		return
	}

	if !strings.HasPrefix(res.Node.Key, path.Join(prefix, jobPrefix)) {
		return
	}

	switch path.Base(res.Node.Key) {
	case "target-state":
		ev = JobTargetStateChangeEvent
		ok = true
	case "target":
		ev = JobTargetChangeEvent
		ok = true
	}

	return
}

func watch(client etcd.Client, key string, stop chan struct{}) (res *etcd.Result) {
	for res == nil {
		select {
		case <-stop:
			log.V(1).Infof("Gracefully closing etcd watch loop: key=%s", key)
			return
		default:
			req := &etcd.Watch{
				Key:       key,
				WaitIndex: 0,
				Recursive: true,
			}

			log.V(1).Infof("Creating etcd watcher: %v", req)

			var err error
			res, err = client.Wait(req, stop)
			if err != nil {
				log.Errorf("etcd watcher %v returned error: %v", req, err)
			}
		}

		// Let's not slam the etcd server in the event that we know
		// an unexpected error occurred.
		time.Sleep(time.Second)
	}

	return
}
