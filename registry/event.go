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
}

func NewEventStream(client *etcd.Client) *EventStream {
	return &EventStream{client}
}

func (self *EventStream) Stream(eventChannel chan *event.Event) {
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
			go self.watch(etcdchan, key)
			go pipe(etcdchan, f, eventChannel)
		}
	}
}

func pipe(etcdchan chan *etcd.Response, translate func(resp *etcd.Response) *event.Event, eventchan chan *event.Event) {
	for true {
		resp := <-etcdchan
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

func (self *EventStream) watch(etcdchan chan *etcd.Response, key string) {
	for true {
		log.V(2).Infof("Creating etcd watcher: key=%s, machines=%s", key, strings.Join(self.etcd.GetCluster(), ","))
		_, err := self.etcd.Watch(key, 0, true, etcdchan, nil)

		var errString string
		if err == nil {
			errString = "N/A"
		} else {
			errString = err.Error()
		}

		log.V(2).Infof("etcd watch exited: key=%s, err=\"%s\"", key, errString)

		time.Sleep(time.Second)
	}
}
