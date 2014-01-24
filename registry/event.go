package registry

import (
	"path"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
)

type EventStream struct {
	etcd *etcd.Client
	stop chan bool
}

func NewEventStream(client *etcd.Client) *EventStream {
	return &EventStream{client, make(chan bool)}
}

func (self *EventStream) Stream(eventchan chan *event.Event) {
	watchMap := map[string][]func(*etcd.Response) *event.Event{
		path.Join(keyPrefix, statePrefix):   []func(*etcd.Response) *event.Event{filterEventJobStatePublished, filterEventJobStateExpired},
		path.Join(keyPrefix, jobPrefix):     []func(*etcd.Response) *event.Event{filterEventJobCreated, filterEventJobScheduled, filterEventJobCancelled},
		path.Join(keyPrefix, machinePrefix): []func(*etcd.Response) *event.Event{self.filterEventMachineUpdated, self.filterEventMachineRemoved},
		path.Join(keyPrefix, requestPrefix): []func(*etcd.Response) *event.Event{filterEventRequestCreated},
		path.Join(keyPrefix, offerPrefix):   []func(*etcd.Response) *event.Event{self.filterEventJobOffered, filterEventJobBidSubmitted},
	}

	for key, funcs := range watchMap {
		for _, f := range funcs {
			etcdchan := make(chan *etcd.Response)
			go watch(self.etcd, etcdchan, key, self.stop)
			go pipe(etcdchan, f, eventchan, self.stop)
		}
	}
}

func pipe(etcdchan chan *etcd.Response, translate func(resp *etcd.Response) *event.Event, eventchan chan *event.Event, stopchan chan bool) {
	for true {
		select {
		case <-stopchan:
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

func watch(client *etcd.Client, etcdchan chan *etcd.Response, key string, stopchan chan bool) {
	idx := uint64(0)
	for true {
		select {
		case <-stopchan:
			log.V(2).Infof("Gracefully closing etcd watch loop: key=%s", key)
			return
		default:
			log.V(2).Infof("Creating etcd watcher: key=%s, machines=%s", key, strings.Join(client.GetCluster(), ","))
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
