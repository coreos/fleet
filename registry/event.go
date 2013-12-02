package registry

import (
	"log"
	"path"

	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	EventJobCreated int = iota
	EventJobDeleted
	EventMachineCreated
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
	etcd := etcd.NewClient(nil)
	return &EventStream{etcd: etcd}
}

func (self *EventStream) RegisterMachineJobEventListener(eventchan chan Event, m *machine.Machine) {
	key := path.Join(keyPrefix, machinePrefix, m.BootId, schedulePrefix)
	self.registerJobEventGenerator(eventchan, key)
}

func (self *EventStream) RegisterGlobalEventListener(eventchan chan Event) {
	self.registerMachineEventGenerator(eventchan)
	self.registerRequestEventGenerator(eventchan)

	key := path.Join(keyPrefix, scheduleAllPrefix)
	self.registerJobEventGenerator(eventchan, key)
}

func (self *EventStream) registerJobEventGenerator(eventchan chan Event, key string) {
	etcdchan := make(chan *etcd.Response)

	eventTranslater := func() {
		for true {
			resp := <-etcdchan
			log.Println("Registry JobEventGenerator etcd watcher got event")

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
				continue
			}

			var jp job.JobPayload
			err := unmarshal(value, &jp)
			if err != nil {
				log.Printf("Failed to deserialize payload for job '%s'", name)
				continue
			}

			j, _ := job.NewJob(name, nil, &jp)
			event := Event{eventType, *j}

			eventchan <- event
		}
	}

	go eventTranslater()
	go self.etcd.Watch(key, 0, true, etcdchan, nil)
}

func (self *EventStream) registerMachineEventGenerator(eventchan chan Event) {
	etcdchan := make(chan *etcd.Response)

	eventTranslater := func() {
		for true {
			resp := <-etcdchan
			log.Println("Registry MachineEventGenerator etcd watcher got event")

			dir, base := path.Split(resp.Key)
			if base != "addrs" {
				continue
			}

			var eventType int
			if resp.Action == "set" && resp.PrevValue == "" {
				eventType = EventMachineCreated
			} else if resp.Action == "expire" || resp.Action == "delete" {
				eventType = EventMachineDeleted
			} else {
				continue
			}

			name := path.Base(dir)
			m := machine.New(name)
			event := Event{eventType, *m}

			eventchan <- event
		}
	}

	go eventTranslater()

	key := path.Join(keyPrefix, machinePrefix)
	go self.etcd.Watch(key, 0, true, etcdchan, nil)
}

func (self *EventStream) registerRequestEventGenerator(eventchan chan Event) {
	etcdchan := make(chan *etcd.Response)

	eventTranslater := func() {
		for true {
			resp := <-etcdchan
			log.Println("Registry RequestEventGenerator etcd watcher got event")

			var eventType int
			if resp.Action == "set" && resp.PrevValue == "" {
				eventType = EventRequestCreated
			} else {
				continue
			}

			var request job.JobRequest
			if err := unmarshal(resp.Value, &request); err != nil {
				log.Print(err)
			}

			event := Event{eventType, request}
			eventchan <- event
		}
	}

	go eventTranslater()

	key := path.Join(keyPrefix, requestPrefix)
	go self.etcd.Watch(key, 0, true, etcdchan, nil)
}
