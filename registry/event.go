package registry

import (
	"path"
	"strings"
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
)

type EventStream struct {
	etcd     *etcd.Client
	registry *Registry
	close    chan bool
}

func NewEventStream(client *etcd.Client, registry *Registry) *EventStream {
	return &EventStream{client, registry, make(chan bool)}
}

func (self *EventStream) Stream(idx uint64, eventchan chan *event.Event) {
	watchMap := map[string][]func(*etcd.Response) *event.Event{
		path.Join(keyPrefix, jobPrefix):     []func(*etcd.Response) *event.Event{filterEventJobCreated, filterEventJobScheduled, filterEventJobStopped, self.filterEventJobUpdated},
		path.Join(keyPrefix, machinePrefix): []func(*etcd.Response) *event.Event{self.filterEventMachineCreated, self.filterEventMachineRemoved},
		path.Join(keyPrefix, offerPrefix):   []func(*etcd.Response) *event.Event{self.filterEventJobOffered, filterEventJobBidSubmitted},
	}

	for key, funcs := range watchMap {
		for _, f := range funcs {
			etcdchan := make(chan *etcd.Response)
			go watch(self.etcd, idx, etcdchan, key, self.close)
			go pipe(etcdchan, f, eventchan, self.close)
		}
	}
}

func (self *EventStream) Close() {
	log.V(1).Info("Closing EventStream")
	close(self.close)
}

func pipe(etcdchan chan *etcd.Response, translate func(resp *etcd.Response) *event.Event, eventchan chan *event.Event, closechan chan bool) {
	for true {
		select {
		case <-closechan:
			return
		case resp := <-etcdchan:
			log.V(2).Infof("Received response from etcd watcher: Action=%s ModifiedIndex=%d Key=%s", resp.Action, resp.Node.ModifiedIndex, resp.Node.Key)
			ev := translate(resp)
			if ev != nil {
				log.V(2).Infof("Translated response(ModifiedIndex=%d) to event(Type=%s)", resp.Node.ModifiedIndex, ev.Type)
				eventchan <- ev
			} else {
				log.V(2).Infof("Discarding response(ModifiedIndex=%d) from etcd watcher", resp.Node.ModifiedIndex)
			}
		}
	}
}

func watch(client *etcd.Client, idx uint64, etcdchan chan *etcd.Response, key string, closechan chan bool) {
	for true {
		select {
		case <-closechan:
			log.V(2).Infof("Gracefully closing etcd watch loop: key=%s", key)
			return
		default:
			log.V(2).Infof("Creating etcd watcher: key=%s, index=%d, machines=%s", key, idx, strings.Join(client.GetCluster(), ","))
			resp, err := client.Watch(key, idx, true, nil, nil)

			if err == nil {
				idx = resp.Node.ModifiedIndex + 1
				etcdchan <- resp
			} else {
				log.V(2).Infof("etcd watcher returned error: key=%s, err=\"%s\"", key, err.Error())

				// Let's not slam the etcd server in the event that we know
				// an unexpected error occurred.
				time.Sleep(time.Second)
			}
		}
	}
}
