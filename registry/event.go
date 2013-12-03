package registry

import (
	"log"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	EventJobCreated int = iota
	EventJobDeleted
	EventJobWatchCreated
	EventJobWatchDeleted
	EventJobWatchClaimExpired
	EventMachineCreated
	EventMachineUpdated
	EventMachineDeleted
	EventRequestCreated
)

type Event struct {
	Type    int
	Payload interface{}
}

type EventStream struct {
	etcd      *etcd.Client
	consumers []chan Event
}

func NewEventStream() *EventStream {
	client := etcd.NewClient(nil)
	client.SetConsistency(etcd.WEAK_CONSISTENCY)
	return &EventStream{etcd: client}
}

func (self *EventStream) RegisterJobEventListener(eventchan chan Event, m *machine.Machine) {
	translate := func(resp *etcd.Response) *Event {
		name := path.Base(resp.Key)

		var eventType int
		var value string

		if resp.Action == "set" && resp.PrevValue == "" {
			eventType = EventJobCreated
			value = resp.Value
		} else if resp.Action == "expire" || resp.Action == "delete" {
			eventType = EventJobDeleted
			value = resp.PrevValue
		} else {
			return nil
		}

		var jp job.JobPayload
		err := unmarshal(value, &jp)
		if err != nil {
			log.Printf("Failed to deserialize payload for job '%s'", name)
			return nil
		}

		j, _ := job.NewJob(name, nil, &jp)
		return &Event{eventType, *j}
	}

	etcdchan := make(chan *etcd.Response)
	go pipe(etcdchan, translate, eventchan)

	key := path.Join(keyPrefix, machinePrefix, m.BootId, schedulePrefix)
	go self.watch(etcdchan, key)
}

func (self *EventStream) registerJobWatchEventGenerator(eventchan chan Event) {
	translate := func(resp *etcd.Response) *Event {
		var eventType int
		var value string

		if resp.Action == "set" && resp.PrevValue == "" {
			eventType = EventJobWatchCreated
			value = resp.Value
		} else if resp.Action == "delete" {
			eventType = EventJobWatchDeleted
			value = resp.PrevValue
		} else if resp.Action == "expire" {
			eventType = EventJobWatchClaimExpired
			value = resp.PrevValue
		} else {
			return nil
		}

		var jw job.JobWatch
		err := unmarshal(value, &jw)
		if err != nil {
			log.Printf("Failed to deserialize JobWatch")
			return nil
		}

		return &Event{eventType, jw}
	}

	etcdchan := make(chan *etcd.Response)
	go pipe(etcdchan, translate, eventchan)

	key := path.Join(keyPrefix, jobWatchPrefix)
	go self.watch(etcdchan, key)
}

func (self *EventStream) registerMachineEventGenerator(eventchan chan Event) {
	translate := func(resp *etcd.Response) *Event {
		dir, base := path.Split(resp.Key)
		if base != "addrs" {
			return nil
		}

		var eventType int
		if resp.Action == "set" && resp.PrevValue == "" {
			eventType = EventMachineCreated
		} else if resp.Action == "set" && resp.PrevValue != "" {
			eventType = EventMachineUpdated
		} else if resp.Action == "expire" || resp.Action == "delete" {
			eventType = EventMachineDeleted
		} else {
			return nil
		}

		name := path.Base(dir)
		m := machine.New(name)
		return &Event{eventType, *m}
	}

	etcdchan := make(chan *etcd.Response)
	go pipe(etcdchan, translate, eventchan)

	key := path.Join(keyPrefix, machinePrefix)
	go self.watch(etcdchan, key)
}

func (self *EventStream) registerRequestEventGenerator(eventchan chan Event) {
	translate := func(resp *etcd.Response) *Event {
		var eventType int
		if resp.Action == "set" && resp.PrevValue == "" {
			eventType = EventRequestCreated
		} else {
			return nil
		}

		var request job.JobRequest
		if err := unmarshal(resp.Value, &request); err != nil {
			log.Print(err)
			return nil
		}

		return &Event{eventType, request}
	}

	etcdchan := make(chan *etcd.Response)
	go pipe(etcdchan, translate, eventchan)

	key := path.Join(keyPrefix, requestPrefix)
	go self.watch(etcdchan, key)
}

func (self *EventStream) RegisterGlobalEventListener(eventchan chan Event) {
	self.registerMachineEventGenerator(eventchan)
	self.registerRequestEventGenerator(eventchan)
	self.registerJobWatchEventGenerator(eventchan)
}

func pipe(etcdchan chan *etcd.Response, translate func(resp *etcd.Response) *Event, eventchan chan Event) {
	for true {
		resp := <-etcdchan
		log.Printf("Received response from etcd watcher: Action=%s ModifiedIndex=%d Key=%s", resp.Action, resp.ModifiedIndex, resp.Key)
		event := translate(resp)
		if event != nil {
			log.Printf("Translated response(ModifiedIndex=%d) to event(Type=%d)", resp.ModifiedIndex, event.Type)
			eventchan <- *event
		} else {
			log.Printf("Discarding response(ModifiedIndex=%d) from etcd watcher", resp.ModifiedIndex)
		}
	}
}

func (self *EventStream) watch(etcdchan chan *etcd.Response, key string) {
	for true {
		log.Printf("Creating etcd watcher: key=%s, machines=%s", key, strings.Join(self.etcd.GetCluster(), ","))
		_, err := self.etcd.Watch(key, 0, true, etcdchan, nil)

		var errString string
		if err == nil {
			errString = "N/A"
		} else {
			errString = err.Error()
		}

		log.Printf("etcd watch exited: key=%s, err=\"%s\"", key, errString)

		time.Sleep(time.Second)
	}
}
