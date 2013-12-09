package registry

import (
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	log "github.com/golang/glog"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

type Event struct {
	Type    string
	Payload interface{}
	Context *machine.Machine
}

type EventStream struct {
	etcd      *etcd.Client
	listeners []EventListener
	chClose   chan bool
}

type EventListener struct {
	Handler  interface{}
	Context   *machine.Machine
}

func NewEventStream() *EventStream {
	client := etcd.NewClient(nil)
	client.SetConsistency(etcd.WEAK_CONSISTENCY)
	listeners := make([]EventListener, 0)

	return &EventStream{client, listeners, make(chan bool)}
}

func (self *EventStream) RegisterListener(l interface{}, m *machine.Machine) {
	self.listeners = append(self.listeners, EventListener{l, m})
}

// Distribute an event to all listeners registered to event.Type
func (self *EventStream) send(event Event) {
	handlerFuncName := fmt.Sprintf("Handle%s", event.Type)
	for _, listener := range self.listeners {
		if event.Context == nil || event.Context.BootId == listener.Context.BootId {
			handlerFunc := reflect.ValueOf(listener.Handler).MethodByName(handlerFuncName)
			if handlerFunc.IsValid() {
				go handlerFunc.Call([]reflect.Value{reflect.ValueOf(event)})
			}
		}
	}
}

func (self *EventStream) eventLoop(event chan Event, stop chan bool) {
	for {
		select {
		case <-stop:
			return
		case e := <-event:
			self.send(e)
		}
	}
}

func (self *EventStream) Open() {
	eventchan := make(chan Event)

	watchMap := map[string][]func(*etcd.Response) *Event {
		path.Join(keyPrefix, statePrefix):
			[]func(*etcd.Response) *Event {filterEventJobStatePublished, filterEventJobStateExpired},
		path.Join(keyPrefix, machinePrefix):
			[]func(*etcd.Response) *Event {filterEventMachineCreated, filterEventMachineUpdated, filterEventMachineDeleted, filterEventJobCreated, filterEventJobDeleted},
		path.Join(keyPrefix, requestPrefix):
			[]func(*etcd.Response) *Event {filterEventRequestCreated},
		path.Join(keyPrefix, jobWatchPrefix):
			[]func(*etcd.Response) *Event {filterEventJobWatchCreated, filterEventJobWatchDeleted, self.filterEventJobWatchClaimExpired},
	}

	for key, funcs := range watchMap {
		for _, f := range funcs {
			etcdchan := make(chan *etcd.Response)
			go self.watch(etcdchan, key)
			go pipe(etcdchan, f, eventchan)
		}
	}

	go self.eventLoop(eventchan, self.chClose)
}

func (self *EventStream) Close() {
	self.chClose <- true
}

func filterEventJobCreated(resp *etcd.Response) *Event {
	if resp.Action != "set" || resp.Node.PrevValue != "" {
		return nil
	}

	dir, jobName := path.Split(resp.Node.Key)
	dir = strings.TrimSuffix(dir, "/")
	dir, prefix := path.Split(dir)
	if prefix != strings.Trim(schedulePrefix, "/") {
		return nil
	}

	var jp job.JobPayload
	err := unmarshal(resp.Node.Value, &jp)
	if err != nil {
		log.V(1).Infof("Failed to deserialize JobPayload: %s", err)
		return nil
	}

	j, _ := job.NewJob(jobName, nil, &jp)

	dir = strings.TrimSuffix(dir, "/")
	dir, machName := path.Split(dir)
	m := machine.New(machName)

	return &Event{"EventJobCreated", *j, m}
}

func filterEventJobDeleted(resp *etcd.Response) *Event {
	if resp.Action != "expire" && resp.Action != "delete" {
		return nil
	}

	dir, jobName := path.Split(resp.Node.Key)
	dir = strings.TrimSuffix(dir, "/")
	dir, prefix := path.Split(dir)
	if prefix != strings.Trim(schedulePrefix, "/") {
		return nil
	}

	var jp job.JobPayload
	err := unmarshal(resp.Node.PrevValue, &jp)
	if err != nil {
		log.V(1).Infof("Failed to deserialize JobPayload: %s", err)
		return nil
	}

	j, _ := job.NewJob(jobName, nil, &jp)

	dir = strings.TrimSuffix(dir, "/")
	dir, machName := path.Split(dir)
	m := machine.New(machName)

	return &Event{"EventJobDeleted", *j, m}
}

func filterEventJobWatchCreated(resp *etcd.Response) *Event {
	if base := path.Base(resp.Node.Key); base != "object" {
		return nil
	}

	if resp.Action != "set" || resp.Node.PrevValue != "" {
		return nil
	}

	var jw job.JobWatch
	err := unmarshal(resp.Node.Value, &jw)
	if err != nil {
		log.V(1).Infof("Failed to deserialize JobWatch: %s", err)
		return nil
	}

	return &Event{"EventJobWatchCreated", jw, nil}
}

func filterEventJobWatchDeleted(resp *etcd.Response) *Event {
	if base := path.Base(resp.Node.Key); base != "object" {
		return nil
	}

	if resp.Action != "delete" {
		return nil
	}

	name := path.Base(path.Dir(resp.Node.Key))
	return &Event{"EventJobWatchDeleted", name, nil}
}

func (self *EventStream) filterEventJobWatchClaimExpired(resp *etcd.Response) *Event {
	if base := path.Base(resp.Node.Key); base != "object" {
		return nil
	}

	if resp.Action != "delete" {
		return nil
	}

	jwKey := path.Join(path.Dir(resp.Node.Key), "object")
	jwResp, err := self.etcd.Get(jwKey, false, true)
	if err != nil {
		return nil
	}

	var jw job.JobWatch
	err = unmarshal(jwResp.Node.Value, &jw)
	if err != nil {
		log.V(1).Infof("Failed to deserialize JobWatch: %s", err)
		return nil
	}

	return &Event{"EventJobWatchClaimExpired", jw, nil}
}

func filterEventJobStatePublished(resp *etcd.Response) *Event {
	if resp.Action != "set" {
		return nil
	}

	var js job.JobState
	err := unmarshal(resp.Node.Value, &js)
	if err != nil {
		log.V(1).Infof("Failed to deserialize JobState: %s", err)
		return nil
	}

	//TODO: handle error returned by NewJob
	j, _ := job.NewJob(path.Base(resp.Node.Key), &js, nil)
	return &Event{"EventJobStatePublished", *j, nil}
}

func filterEventJobStateExpired(resp *etcd.Response) *Event {
	if resp.Action != "delete" && resp.Action != "expire" {
		return nil
	}

	var js job.JobState
	err := unmarshal(resp.Node.Value, &js)
	if err != nil {
		log.V(1).Infof("Failed to deserialize JobState: %s", err)
		return nil
	}

	//TODO: handle error returned by NewJob
	j, _ := job.NewJob(path.Base(resp.Node.Key), &js, nil)
	return &Event{"EventJobStateExpired", *j, nil}
}

func filterEventMachineCreated(resp *etcd.Response) *Event {
	if base := path.Base(resp.Node.Key); base != "addrs" {
		return nil
	}

	if resp.Action != "set" || resp.Node.PrevValue != "" {
		return nil
	}

	name := path.Base(path.Dir(resp.Node.Key))
	m := machine.New(name)
	return &Event{"EventMachineCreated", *m, nil}
}

func filterEventMachineUpdated(resp *etcd.Response) *Event {
	if base := path.Base(resp.Node.Key); base != "addrs" {
		return nil
	}

	if resp.Action != "set" || resp.Node.PrevValue == "" {
		return nil
	}

	name := path.Base(path.Dir(resp.Node.Key))
	m := machine.New(name)
	return &Event{"EventMachineUpdated", *m, nil}
}

func filterEventMachineDeleted(resp *etcd.Response) *Event {
	if base := path.Base(resp.Node.Key); base != "addrs" {
		return nil
	}

	if resp.Action != "expire" && resp.Action != "delete" {
		return nil
	}

	name := path.Base(path.Dir(resp.Node.Key))
	m := machine.New(name)
	return &Event{"EventMachineDeleted", *m, nil}
}

func filterEventRequestCreated(resp *etcd.Response) *Event{
	if resp.Action != "set" || resp.Node.PrevValue != "" {
		return nil
	}

	var request job.JobRequest
	if err := unmarshal(resp.Node.Value, &request); err != nil {
		log.V(1).Infof("Failed to deserialize JobRequest: %s", err)
		return nil
	}

	return &Event{"EventRequestCreated", request, nil}
}

func pipe(etcdchan chan *etcd.Response, translate func(resp *etcd.Response) *Event, eventchan chan Event) {
	for true {
		resp := <-etcdchan
		log.V(2).Infof("Received response from etcd watcher: Action=%s ModifiedIndex=%d Key=%s", resp.Action, resp.Node.ModifiedIndex, resp.Node.Key)
		event := translate(resp)
		if event != nil {
			log.V(2).Infof("Translated response(ModifiedIndex=%d) to event(Type=%s)", resp.Node.ModifiedIndex, event.Type)
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
