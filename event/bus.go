package event

import (
	"sync"
)

type listenerFunc func()

type EventBus struct {
	lFuncMap map[Event][]listenerFunc
}

func NewEventBus() *EventBus {
	return &EventBus{
		lFuncMap: make(map[Event][]listenerFunc),
	}
}

func (eb *EventBus) AddListener(eType Event, lFunc listenerFunc) {
	lFuncs, ok := eb.lFuncMap[eType]
	if !ok {
		lFuncs = make([]listenerFunc, 0)
	}

	eb.lFuncMap[eType] = append(lFuncs, lFunc)
}

// Dispatch calls all listeners registered to the given Event
func (eb *EventBus) Dispatch(ev Event) {
	wg := sync.WaitGroup{}
	for _, lFunc := range eb.lFuncMap[ev] {
		wg.Add(1)
		go func() {
			lFunc()
			wg.Done()
		}()
	}
	wg.Wait()
}
