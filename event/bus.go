package event

import (
	"fmt"
	"reflect"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/machine"
)

type EventBus struct {
	listeners map[string]EventListener
	Channel   chan *Event
	stop      chan bool
}

func NewEventBus() *EventBus {
	listeners := make(map[string]EventListener, 0)
	return &EventBus{listeners, make(chan *Event), make(chan bool)}
}

func (self *EventBus) Listen() {
	go func() {
		for {
			select {
			case <-self.stop:
				return
			case ev := <-self.Channel:
				self.dispatch(ev)
			}
		}
	}()
}

func (self *EventBus) Stop() {
	log.V(1).Info("Stopping EventBus")
	close(self.stop)
}

func (self *EventBus) AddListener(name string, m *machine.Machine, l interface{}) {
	listener := EventListener{m, l}
	key := fmt.Sprintf("%s-%s", name, m.String())
	self.listeners[key] = listener
}

func (self *EventBus) RemoveListener(name string, m *machine.Machine) {
	key := fmt.Sprintf("%s-%s", name, m.String())
	if _, ok := self.listeners[key]; ok {
		delete(self.listeners, key)
	}
}

// Distribute an Event to all listeners registered to Event.Type
func (self *EventBus) dispatch(ev *Event) {
	log.V(2).Infof("Dispatching %s to listeners", ev.Type)
	handlerFuncName := fmt.Sprintf("Handle%s", ev.Type)
	for _, listener := range self.listeners {
		log.V(2).Infof("Looking for event handler func %s on listener %s", handlerFuncName, listener.String())
		handlerFunc := reflect.ValueOf(listener.Handler).MethodByName(handlerFuncName)
		if handlerFunc.IsValid() {
			log.V(2).Infof("Calling event handler for %s on listener %s", ev.Type, listener.String())
			go handlerFunc.Call([]reflect.Value{reflect.ValueOf(*ev)})
		}
	}
}
