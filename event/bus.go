package event

import (
	"fmt"
	"reflect"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

type EventBus struct {
	listeners map[string]interface{}
	Channel   chan *Event
}

func NewEventBus() *EventBus {
	listeners := make(map[string]interface{}, 0)
	return &EventBus{listeners, make(chan *Event)}
}

func (self *EventBus) Listen(stop chan bool) {
	go func() {
		for {
			select {
			case <-stop:
				return
			case ev := <-self.Channel:
				self.dispatch(ev)
			}
		}
	}()
}

func (self *EventBus) AddListener(name string, l interface{}) {
	self.listeners[name] = l
}

func (self *EventBus) RemoveListener(name string) {
	delete(self.listeners, name)
}

// Distribute an Event to all listeners registered to Event.Type
func (self *EventBus) dispatch(ev *Event) {
	log.V(1).Infof("Dispatching %s to listeners", ev.Type)
	handlerFuncName := fmt.Sprintf("Handle%s", ev.Type)
	for name, listener := range self.listeners {
		log.V(1).Infof("Looking for event handler func %s on listener %s", handlerFuncName, name)
		handlerFunc := reflect.ValueOf(listener).MethodByName(handlerFuncName)
		if handlerFunc.IsValid() {
			log.V(1).Infof("Calling event handler for %s on listener %s", ev.Type, name)
			go handlerFunc.Call([]reflect.Value{reflect.ValueOf(*ev)})
		}
	}
}
