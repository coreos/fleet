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
		ev *event.Event
	}{
		{
			in: nil,
			ev: nil,
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/"}},
			ev: nil,
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/fleet"}},
			ev: &event.GlobalEvent,
		},
		{
			in: &etcd.Result{Node: &etcd.Node{Key: "/fleet/job"}},
			ev: &event.JobEvent,
		},
	}

	for i, tt := range tests {
		etcdchan := make(chan *etcd.Result)
		stopchan := make(chan bool)
		prefix := "/fleet"

		send := func(ev *event.Event) {
			if !reflect.DeepEqual(tt.ev, ev) {
				t.Errorf("case %d: received incorrect event\nexpected %#v\ngot %#v", i, tt.ev, ev)
			}
		}

		go filter(etcdchan, prefix, send, stopchan)

		etcdchan <- tt.in
		close(stopchan)
	}
}
