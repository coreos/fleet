package agent

import (
	"github.com/coreos/fleet/event"
)

type EventHandler struct {
	ar *AgentReconciler
}

func NewEventHandler(ar *AgentReconciler) *EventHandler {
	return &EventHandler{ar}
}

func (eh *EventHandler) HandleEventJobOffered(ev event.Event) {
	eh.ar.Trigger()
}

func (eh *EventHandler) HandleEventJobScheduled(ev event.Event) {
	eh.ar.Trigger()
}

func (eh *EventHandler) HandleJobTargetStateStarted(ev event.Event) {
	eh.ar.Trigger()
}

func (eh *EventHandler) HandleJobTargetStateStopped(ev event.Event) {
	eh.ar.Trigger()
}

func (eh *EventHandler) HandleEventJobUnscheduled(ev event.Event) {
	eh.ar.Trigger()
}

func (eh *EventHandler) HandleEventJobDestroyed(ev event.Event) {
	eh.ar.Trigger()
}
