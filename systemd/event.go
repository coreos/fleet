package systemd

import (
	"github.com/coreos/go-systemd/dbus"
	log "github.com/golang/glog"

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

func (es *EventStream) Stream(sendFunc func(*event.Event), stop chan bool) {

	es.mgr.systemd.Subscribe()
	defer es.mgr.systemd.Unsubscribe()
	changechan, errchan := es.mgr.subscriptions.Subscribe()

	for {
		select {
		case <-stop:
			return
		case err := <-errchan:
			log.Errorf("Received error from dbus: err=%v", err)
		case changes := <-changechan:
			log.V(1).Infof("Received event from dbus")
			events := translateUnitStatusEvents(changes)
			for i := range events {
				ev := events[i]
				log.V(1).Infof("Translated dbus event to event(Type=%s)", ev.Type)
				sendFunc(&ev)
			}
		}
	}
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
