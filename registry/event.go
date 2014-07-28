package registry

import (
	"errors"
	"path"
	"strings"
	"time"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"

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
	etcdchan := make(chan *etcd.Result)
	go watch(es.etcd, idx, etcdchan, es.registry.keyPrefix, stop)
	go filter(etcdchan, es.registry.keyPrefix, sendFunc, stop)
}

func filter(etcdchan chan *etcd.Result, prefix string, sendFunc func(*event.Event), stop chan bool) {
	parse := func(res *etcd.Result) *event.Event {
		if res == nil || res.Node == nil {
			return nil
		}

		if !strings.HasPrefix(res.Node.Key, prefix) {
			return nil
		}

		var ev event.Event
		if strings.HasPrefix(res.Node.Key, path.Join(prefix, jobPrefix)) {
			ev = event.JobEvent
		} else {
			ev = event.GlobalEvent
		}

		return &ev
	}

	for {
		select {
		case <-stop:
			return
		case res := <-etcdchan:
			log.V(1).Infof("Received %v from etcd watch", res)

			ev := parse(res)
			if ev == nil {
				continue
			}

			log.V(1).Infof("Translated %v to Event(Type=%s)", res, ev)
			sendFunc(ev)
		}
	}
}

func watch(client etcd.Client, idx uint64, etcdchan chan *etcd.Result, key string, stop chan bool) {
	for {
		select {
		case <-stop:
			log.V(1).Infof("Gracefully closing etcd watch loop: key=%s", key)
			return
		default:
			req := &etcd.Watch{
				Key:       key,
				WaitIndex: idx,
				Recursive: true,
			}

			log.V(1).Infof("Creating etcd watcher: %v", req)

			resp, err := client.Wait(req, stop)
			if err == nil {
				if resp.Node != nil {
					idx = resp.Node.ModifiedIndex + 1
				}
				etcdchan <- resp
				continue
			}

			log.Errorf("etcd watcher %v returned error: %v", req, err)

			etcdError, ok := err.(etcd.Error)
			if !ok {
				// Let's not slam the etcd server in the event that we know
				// an unexpected error occurred.
				time.Sleep(time.Second)
				continue
			}

			switch etcdError.ErrorCode {
			case etcd.ErrorEventIndexCleared:
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
