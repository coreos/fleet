package registry

import (
	"reflect"
	"testing"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/event"
)

func TestFilterEtcdEvents(t *testing.T) {
	tests := []struct {
		in string
		ev []event.Event
	}{
		{
			in: "",
			ev: []event.Event{},
		},
		{
			in: "/",
			ev: []event.Event{},
		},
		{
			in: "/fleet",
			ev: []event.Event{},
		},
		{
			in: "/fleet/job",
			ev: []event.Event{},
		},
		{
			in: "/fleet/job/foo/object",
			ev: []event.Event{},
		},
		{
			in: "/fleet/machine/asdf",
			ev: []event.Event{},
		},
		{
			in: "/fleet/state/asdf",
			ev: []event.Event{},
		},
		{
			in: "/fleet/job/asdf/target-state",
			ev: []event.Event{event.JobTargetStateChangeEvent},
		},
		{
			in: "/fleet/job/foobarbaz/target-state",
			ev: []event.Event{event.JobTargetStateChangeEvent},
		},
		{
			in: "/fleet/job/asdf/target",
			ev: []event.Event{event.JobTargetChangeEvent},
		},
	}

	for i, tt := range tests {
		for _, action := range []string{"set", "update", "create", "delete"} {
			etcdchan := make(chan *etcd.Result)
			stopchan := make(chan bool)
			prefix := "/fleet"

			got := make([]event.Event, 0)
			send := func(ev event.Event) {
				got = append(got, ev)
			}

			go filter(etcdchan, prefix, send, stopchan)

			var res *etcd.Result
			if tt.in != "" {
				res = &etcd.Result{
					Node: &etcd.Node{
						Key: tt.in,
					},
					Action: action,
				}
			}
			etcdchan <- res

			if !reflect.DeepEqual(tt.ev, got) {
				t.Errorf("case %d: received incorrect event\nexpected %#v\ngot %#v", i, tt.ev, got)
				t.Logf("action: %v", action)
			}

			close(stopchan)
		}
	}
}
