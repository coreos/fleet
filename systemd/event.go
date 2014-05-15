package systemd

import (
	"github.com/coreos/fleet/third_party/github.com/coreos/go-systemd/dbus"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/unit"
)

type EventStream struct {
	mgr   *SystemdUnitManager
	close chan bool
}

func NewEventStream(mgr *SystemdUnitManager) *EventStream {
	return &EventStream{mgr, nil}
}

func (es *EventStream) Stream(eventchan chan *event.Event, stop chan bool) {

	es.mgr.systemd.Subscribe()
	changechan, errchan := es.mgr.subscriptions.Subscribe()

	for true {
		select {
		case <-stop:
			break
		case err := <-errchan:
			log.Errorf("Received error from dbus: err=%v", err)
		case changes := <-changechan:
			log.V(1).Infof("Received event from dbus")
			events := translateUnitStatusEvents(changes)
			for i, _ := range events {
				ev := events[i]
				log.V(1).Infof("Translated dbus event to event(Type=%s)", ev.Type)
				eventchan <- &ev
			}
		}
	}

	es.mgr.systemd.Unsubscribe()
}

func translateUnitStatusEvents(changes map[string]*dbus.UnitStatus) []event.Event {
	events := make([]event.Event, 0)
	for name, status := range changes {
		var state *unit.UnitState
		if status != nil {
			state = unit.NewUnitState(status.LoadState, status.ActiveState, status.SubState, nil)
		}
		ev := event.Event{"EventUnitStateUpdated", state, name}
		events = append(events, ev)
	}
	return events
}
