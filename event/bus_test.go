package event

import (
	"testing"
	"time"
)

type TestListener struct {
	evchan chan Event
}

func (l *TestListener) HandleEventTypeOne(ev Event) {
	l.evchan <- ev
}

func TestEventBus(t *testing.T) {
	evchan := make(chan Event)

	bus := NewEventBus()
	bus.AddListener("test", "X", &TestListener{evchan})
	bus.Listen()
	defer bus.Stop()

	ev := Event{"EventTypeOne", "payload", "Y"}
	bus.Channel <- &ev

	select {
	case <-time.After(time.Second):
		t.Fatalf("Failed to dispatch event within a second")
	case recv := <-evchan:
		if recv.Payload.(string) != "payload" {
			t.Error("event payload is incorrect")
		}
		if recv.Context.(string) != "Y" {
			t.Error("event context is incorrect")
		}
	}
}

func TestEventBusNoDispatch(t *testing.T) {
	evchan := make(chan Event)

	bus := NewEventBus()
	bus.AddListener("test", "X", &TestListener{evchan})
	bus.Listen()
	defer bus.Stop()

	go func() {
		ev := Event{"EventTypeTwo", "payload", "Y"}
		bus.Channel <- &ev

		ev = Event{"EventTypeOne", "payload", "Y"}
		bus.Channel <- &ev
	}()

	recv := <-evchan
	if recv.Type != "EventTypeOne" {
		t.Fatalf("handler received unexpected event")
	}
}
