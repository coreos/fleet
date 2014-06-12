package registry

import (
	"errors"
	"strings"
	"time"

	goetcd "github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
)

type EventStream struct {
	etcd     etcd.Client
	registry *EtcdRegistry
}

func NewEventStream(client etcd.Client, registry Registry) (*EventStream, error) {
	reg, ok := registry.(*EtcdRegistry)
	if !ok {
		return nil, errors.New("EventStream currently only works with EtcdRegistry")
	}

	return &EventStream{client, reg}, nil
}

func (es *EventStream) Stream(idx uint64, sendFunc func(*event.Event), stop chan bool) {
	filters := []func(*goetcd.Response) *event.Event{
		filterEventJobDestroyed,
		filterEventJobScheduled,
		filterEventJobUnscheduled,
		es.filterJobTargetStateChanges,
		filterEventMachineCreated,
		filterEventMachineRemoved,
		filterEventMachineLost,
		es.filterEventJobOffered,
		filterEventJobBidSubmitted,
	}

	etcdchan := make(chan *goetcd.Response)
	go watch(es.etcd, idx, etcdchan, es.registry.keyPrefix, stop)
	go pipe(etcdchan, filters, sendFunc, stop)
}

func pipe(etcdchan chan *goetcd.Response, filters []func(resp *goetcd.Response) *event.Event, sendFunc func(*event.Event), stop chan bool) {
	for true {
		select {
		case <-stop:
			return
		case resp := <-etcdchan:
			log.V(1).Infof("Received response from etcd watcher: Action=%s ModifiedIndex=%d Key=%s", resp.Action, resp.Node.ModifiedIndex, resp.Node.Key)
			for _, f := range filters {
				ev := f(resp)
				if ev == nil {
					continue
				}

				log.V(1).Infof("Translated response(ModifiedIndex=%d) to event(Type=%s)", resp.Node.ModifiedIndex, ev.Type)
				sendFunc(ev)
			}
		}
	}
}

func watch(client etcd.Client, idx uint64, etcdchan chan *goetcd.Response, key string, stop chan bool) {
	for true {
		select {
		case <-stop:
			log.V(1).Infof("Gracefully closing etcd watch loop: key=%s", key)
			return
		default:
			log.V(1).Infof("Creating etcd watcher: key=%s, index=%d, machines=%s", key, idx, strings.Join(client.GetCluster(), ","))
			resp, err := client.Watch(key, idx, true, nil, stop)

			if err == nil {
				idx = resp.Node.ModifiedIndex + 1
				etcdchan <- resp
				continue
			}

			log.Errorf("etcd watcher returned error: key=%s, err=\"%s\"", key, err.Error())

			etcdError, ok := err.(*goetcd.EtcdError)
			if !ok {
				// Let's not slam the etcd server in the event that we know
				// an unexpected error occurred.
				time.Sleep(time.Second)
				continue
			}

			switch etcdError.ErrorCode {
			case etcd.EcodeEventIndexCleared:
				// This is racy, but adding one to the last known index
				// will help get this watcher back into the range of
				// etcd's internal event history
				idx = idx + 1
			default:
				// Let's not slam the etcd server in the event that we know
				// an unexpected error occurred.
				time.Sleep(time.Second)
			}
		}
	}
}
