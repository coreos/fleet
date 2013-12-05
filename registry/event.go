package registry

import (
	"path"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	log "github.com/golang/glog"

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
		name := path.Base(resp.Node.Key)

		var eventType int
		var value string

		if resp.Action == "set" && resp.Node.PrevValue == "" {
			eventType = EventJobCreated
			value = resp.Node.Value
		} else if resp.Action == "expire" || resp.Action == "delete" {
			eventType = EventJobDeleted
			value = resp.Node.PrevValue
		} else {
			return nil
		}

		var jp job.JobPayload
		err := unmarshal(value, &jp)
		if err != nil {
			log.V(1).Infof("Failed to deserialize JobPayload: %s", err)
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

		dir, base := path.Split(resp.Node.Key)
		if base == "object" {
			if resp.Action == "set" && resp.Node.PrevValue == "" {
				eventType = EventJobWatchCreated
				value = resp.Node.Value
			} else if resp.Action == "delete" {
				eventType = EventJobWatchDeleted
				value = resp.Node.PrevValue
			} else {
				return nil
			}
		} else if base == "lock" {
			if resp.Action == "expire" {
				eventType = EventJobWatchClaimExpired

				resp2, err := self.etcd.Get(path.Join(dir, "object"), false, true)
				if err != nil {
					return nil
				}

				value = resp2.Node.Value
			} else {
				return nil
			}
		}

		var jw job.JobWatch
		err := unmarshal(value, &jw)
		if err != nil {
			log.V(1).Infof("Failed to deserialize JobWatch: %s", err)
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
		dir, base := path.Split(resp.Node.Key)
		if base != "addrs" {
			return nil
		}

		var eventType int
		if resp.Action == "set" && resp.Node.PrevValue == "" {
			eventType = EventMachineCreated
		} else if resp.Action == "set" && resp.Node.PrevValue != "" {
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
		if resp.Action == "set" && resp.Node.PrevValue == "" {
			eventType = EventRequestCreated
		} else {
			return nil
		}

		var request job.JobRequest
		if err := unmarshal(resp.Node.Value, &request); err != nil {
			log.V(1).Infof("Failed to deserialize JobRequest: %s", err)
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
		log.V(2).Infof("Received response from etcd watcher: Action=%s ModifiedIndex=%d Key=%s", resp.Action, resp.Node.ModifiedIndex, resp.Node.Key)
		event := translate(resp)
		if event != nil {
			log.V(2).Infof("Translated response(ModifiedIndex=%d) to event(Type=%d)", resp.Node.ModifiedIndex, event.Type)
			eventchan <- *event
		} else {
			log.V(2).Infof("Discarding response(ModifiedIndex=%d) from etcd watcher", resp.Node.ModifiedIndex)
		}
	}
}

func (self *EventStream) watch(etcdchan chan *etcd.Response, key string) {
	for true {
		log.V(1).Infof("Creating etcd watcher: key=%s, machines=%s", key, strings.Join(self.etcd.GetCluster(), ","))
		_, err := self.etcd.Watch(key, 0, true, etcdchan, nil)

		var errString string
		if err == nil {
			errString = "N/A"
		} else {
			errString = err.Error()
		}

		log.V(1).Infof("etcd watch exited: key=%s, err=\"%s\"", key, errString)

		time.Sleep(time.Second)
	}
}
