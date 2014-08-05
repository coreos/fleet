package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
)

func TestFilterEtcdEvents(t *testing.T) {
	tests := []struct {
		in *etcd.Result
		ev []event.Event
	}{
		{
			in: nil,
			ev: []event.Event{},
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/"}},
			ev: []event.Event{},
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/fleet"}},
			ev: []event.Event{},
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/fleet/job"}},
			ev: []event.Event{event.JobEvent},
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/fleet/job/asdf/target-state"}, Action: "set"},
			ev: []event.Event{event.JobEvent, event.JobTargetStateSetEvent},
		},
	}

	for i, tt := range tests {
		etcdchan := make(chan *etcd.Result)
		stopchan := make(chan bool)
		prefix := "/fleet"

		got := make([]event.Event, 0)
		send := func(ev event.Event) {
			got = append(got, ev)
		}

		go filter(etcdchan, prefix, send, stopchan)

		etcdchan <- tt.in

		if !reflect.DeepEqual(tt.ev, got) {
			t.Errorf("case %d: received incorrect event\nexpected %#v\ngot %#v", i, tt.ev, got)
		}

		close(stopchan)
	}
}
