package registry

import (
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	log "github.com/golang/glog"

	"github.com/coreos/coreinit/event"
	"github.com/coreos/coreinit/machine"
)

type EventStream struct {
	etcd      *etcd.Client
	listeners map[string]event.EventListener
	chClose   chan bool
}

func NewEventStream(client *etcd.Client) *EventStream {
	listeners := make(map[string]event.EventListener, 0)
	return &EventStream{client, listeners, make(chan bool)}
}

func (self *EventStream) AddListener(name string, m *machine.Machine, l interface{}) {
	listener := event.EventListener{m, l}
	key := fmt.Sprintf("%s-%s", name, m.String())
	self.listeners[key] = listener
}

func (self *EventStream) RemoveListener(name string, m *machine.Machine) {
	key := fmt.Sprintf("%s-%s", name, m.String())
	if _, ok := self.listeners[key]; ok {
		delete(self.listeners, key)
	}
}

// Distribute an Event to all listeners registered to Event.Type
func (self *EventStream) send(ev event.Event) {
	log.V(2).Infof("Sending %s to %d listeners", ev.Type, len(self.listeners))
	handlerFuncName := fmt.Sprintf("Handle%s", ev.Type)
	for _, listener := range self.listeners {
		log.V(2).Infof("Looking for func %s on listener %s", handlerFuncName, listener.String())
		handlerFunc := reflect.ValueOf(listener.Handler).MethodByName(handlerFuncName)
		if handlerFunc.IsValid() {
			log.V(2).Infof("Calling event handler for %s on listener %s", ev.Type, listener.String())
			go handlerFunc.Call([]reflect.Value{reflect.ValueOf(ev)})
		}
	}
}

func (self *EventStream) eventLoop(eventChan chan event.Event, stop chan bool) {
	for {
		select {
		case <-stop:
			return
		case ev := <-eventChan:
			self.send(ev)
		}
	}
}

func (self *EventStream) Open() {
	eventchan := make(chan event.Event)

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
			go pipe(etcdchan, f, eventchan)
		}
	}

	go self.eventLoop(eventchan, self.chClose)
}

func (self *EventStream) Close() {
	self.chClose <- true
}

func pipe(etcdchan chan *etcd.Response, translate func(resp *etcd.Response) *event.Event, eventchan chan event.Event) {
	for true {
		resp := <-etcdchan
		log.V(2).Infof("Received response from etcd watcher: Action=%s ModifiedIndex=%d Key=%s", resp.Action, resp.Node.ModifiedIndex, resp.Node.Key)
		event := translate(resp)
		if event != nil {
			log.V(2).Infof("Translated response(ModifiedIndex=%d) to event(Type=%s)", resp.Node.ModifiedIndex, event.Type)
			eventchan <- *event
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
