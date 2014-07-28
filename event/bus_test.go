package event

import (
	"testing"
)

type TestListener struct {
	evchan chan struct{}
}

func (l *TestListener) HandleEvent() {
	go func() { l.evchan <- struct{}{} }()
}

func TestEventBus(t *testing.T) {
	bus := NewEventBus()
	tl := &TestListener{make(chan struct{})}
	ev := Event("TypeOne")
	bus.AddListener(ev, tl.HandleEvent)

	bus.Dispatch(&ev)

	select {
	case <-tl.evchan:
	default:
		t.Fatalf("Failed to dispatch event")
	}
}

func TestEventBusNoDispatch(t *testing.T) {
	bus := NewEventBus()
	tl := &TestListener{make(chan struct{}, 2)}
	ev1 := Event("TypeOne")
	ev2 := Event("TypeTwo")
	bus.AddListener(ev1, tl.HandleEvent)

	bus.Dispatch(&ev2)
	bus.Dispatch(&ev1)

	close(tl.evchan)

	count := 0
	for _ = range tl.evchan {
		count++
	}

	if count != 1 {
		t.Errorf("Expected a single event, got %d", count)
	}
}
