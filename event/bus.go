package event

import (
	"fmt"
	"reflect"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

type EventBus struct {
	listeners map[string]EventListener
	Channel   chan *Event
}

func NewEventBus() *EventBus {
	listeners := make(map[string]EventListener, 0)
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

func (self *EventBus) AddListener(name, bootID string, l interface{}) {
	listener := EventListener{bootID, l}
	key := fmt.Sprintf("%s-%s", name, bootID)
	self.listeners[key] = listener
}

func (self *EventBus) RemoveListener(name, bootID string) {
	key := fmt.Sprintf("%s-%s", name, bootID)
	if _, ok := self.listeners[key]; ok {
		delete(self.listeners, key)
	}
}

// Distribute an Event to all listeners registered to Event.Type
func (self *EventBus) dispatch(ev *Event) {
	log.V(1).Infof("Dispatching %s to listeners", ev.Type)
	handlerFuncName := fmt.Sprintf("Handle%s", ev.Type)
	for _, listener := range self.listeners {
		log.V(1).Infof("Looking for event handler func %s on listener %s", handlerFuncName, listener.String())
		handlerFunc := reflect.ValueOf(listener.Handler).MethodByName(handlerFuncName)
		if handlerFunc.IsValid() {
			log.V(1).Infof("Calling event handler for %s on listener %s", ev.Type, listener.String())
			go handlerFunc.Call([]reflect.Value{reflect.ValueOf(*ev)})
		}
	}
}
