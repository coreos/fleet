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

func (es *EventStream) Stream(sendFunc func(event.Event), stop chan bool) {
	etcdchan := make(chan *etcd.Result)
	go watch(es.etcd, etcdchan, es.registry.keyPrefix, stop)
	go filter(etcdchan, es.registry.keyPrefix, sendFunc, stop)
}

func filter(etcdchan chan *etcd.Result, prefix string, sendFunc func(event.Event), stop chan bool) {
	parse := func(res *etcd.Result) (ev event.Event, ok bool) {
		if res == nil || res.Node == nil {
			return
		}

		// ignore everything but the job namespace
		if !strings.HasPrefix(res.Node.Key, path.Join(prefix, jobPrefix)) {
			return
		}

		_, baseName := path.Split(res.Node.Key)
		switch baseName {
		case "target-state":
			ev = event.JobTargetStateChangeEvent
			ok = true
		case "target":
			ev = event.JobTargetChangeEvent
			ok = true
		default:
		}
		return
	}

	for {
		select {
		case <-stop:
			return
		case res := <-etcdchan:
			log.V(1).Infof("Received %v from etcd watch", res)
			if ev, ok := parse(res); ok {
				log.V(1).Infof("Translated %v to Event(Type=%s)", res, ev)
				sendFunc(ev)
			}
		}
	}
}

func watch(client etcd.Client, etcdchan chan *etcd.Result, key string, stop chan bool) {
	for {
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

			resp, err := client.Wait(req, stop)
			if err != nil {
				log.Errorf("etcd watcher %v returned error: %v", req, err)

				// Let's not slam the etcd server in the event that we know
				// an unexpected error occurred.
				time.Sleep(time.Second)
				continue
			}

			etcdchan <- resp
		}
	}
}
