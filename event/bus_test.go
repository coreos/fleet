package event

import (
	"testing"
	"time"

	"github.com/coreos/fleet/machine"
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
	bus.AddListener("test", machine.New("X", "", make(map[string]string, 0)), &TestListener{evchan})
	bus.Listen()
	defer bus.Stop()

	ev := Event{"EventTypeOne", "payload", machine.New("Y", "", make(map[string]string, 0))}
	bus.Channel <- &ev

	select {
	case <-time.After(time.Second):
		t.Fatalf("Failed to dispatch event within a second")
	case recv := <-evchan:
		if recv.Payload.(string) != "payload" {
			t.Error("event payload is incorrect")
		}
		if recv.Context.(*machine.Machine).State().BootId != "Y" {
			t.Error("event context is incorrect")
		}
	}
}
