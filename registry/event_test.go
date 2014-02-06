package registry

import (
	"testing"

	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/fleet/event"
)

func TestPipe(t *testing.T) {
	etcdchan := make(chan *etcd.Response)

	translate := func(resp *etcd.Response) *event.Event {
		return &event.Event{"TranslateTest", resp.Action, nil}
	}

	eventchan := make(chan *event.Event)
	stopchan := make(chan bool)

	go pipe(etcdchan, translate, eventchan, stopchan)

	resp := etcd.Response{Action: "TestAction", Node: &etcd.Node{Key: "/", ModifiedIndex: 0}}
	etcdchan<- &resp

	ev := <-eventchan
	if ev.Type != "TranslateTest" {
		t.Fatalf("Expected ev.Type 'TranslateTest' but got '%s', ev.Type")
	}

	if ev.Payload.(string) != "TestAction" {
		t.Fatalf("Expected ev.Payload 'TestAction, but got something else")
	}

	if ev.Context != nil {
		t.Fatalf("Expected ev.Context nil, but got non-nil")
	}

	close(stopchan)
}
