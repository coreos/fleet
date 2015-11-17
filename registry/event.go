// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"path"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/pkg"
)

const (
	// Occurs when any Job's target is touched
	JobTargetChangeEvent = pkg.Event("JobTargetChangeEvent")
	// Occurs when any Job's target state is touched
	JobTargetStateChangeEvent = pkg.Event("JobTargetStateChangeEvent")
)

type etcdEventStream struct {
	kAPI       etcd.KeysAPI
	rootPrefix string
}

func NewEtcdEventStream(kAPI etcd.KeysAPI, rootPrefix string) pkg.EventStream {
	return &etcdEventStream{kAPI, rootPrefix}
}

// Next returns a channel which will emit an Event as soon as one of interest occurs
func (es *etcdEventStream) Next(stop chan struct{}) chan pkg.Event {
	evchan := make(chan pkg.Event)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}

			res := watch(es.kAPI, path.Join(es.rootPrefix, jobPrefix), stop)
			if ev, ok := parse(res, es.rootPrefix); ok {
				evchan <- ev
				return
			}
		}

	}()

	return evchan
}

func parse(res *etcd.Response, prefix string) (ev pkg.Event, ok bool) {
	if res == nil || res.Node == nil {
		return
	}

	if !strings.HasPrefix(res.Node.Key, path.Join(prefix, jobPrefix)) {
		return
	}

	switch path.Base(res.Node.Key) {
	case "target-state":
		ev = JobTargetStateChangeEvent
		ok = true
	case "target":
		ev = JobTargetChangeEvent
		ok = true
	}

	return
}

func watch(kAPI etcd.KeysAPI, key string, stop chan struct{}) (res *etcd.Response) {
	for res == nil {
		select {
		case <-stop:
			log.Debugf("Gracefully closing etcd watch loop: key=%s", key)
			return
		default:
			opts := &etcd.WatcherOptions{
				AfterIndex: 0,
				Recursive:  true,
			}
			watcher := kAPI.Watcher(key, opts)
			log.Debugf("Creating etcd watcher: %s", key)

			var err error
			res, err = watcher.Next(context.Background())
			if err != nil {
				log.Errorf("etcd watcher %v returned error: %v", key, err)
			}
		}

		// Let's not slam the etcd server in the event that we know
		// an unexpected error occurred.
		time.Sleep(time.Second)
	}

	return
}
