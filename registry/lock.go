package registry

import (
	"fmt"
	"path"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

const (
	// ResourceMutexTTL is the number of seconds to allow a mutex to be held on a resource
	ResourceMutexTTL = 10

	mutexPrefix = "/mutex/"
)

// lockResource will attempt to lock a mutex on a resource defined by the
// provided class and id. The context will be persisted to the Registry to
// track by whom the mutex is currently locked.
func (r *FleetRegistry) lockResource(class, id, context string) *TimedResourceMutex {
	mutexName := fmt.Sprintf("%s-%s", class, id)
	log.V(1).Infof("Attempting to acquire mutex on %s", mutexName)

	key := path.Join(r.keyPrefix, mutexPrefix, mutexName)
	resp, err := r.storage.Create(key, context, uint64(ResourceMutexTTL))

	if err != nil {
		log.V(1).Infof("Failed to acquire mutex on %s", mutexName)
		return nil
	}

	log.V(1).Infof("Successfully acquired mutex on %s", mutexName)
	return &TimedResourceMutex{r.storage, *resp.Node}
}

// TimedResourceMutex is a proxy to an auto-expiring mutex
// stored in the Registry. It assumes the mutex creator has
// initialized a timer.
type TimedResourceMutex struct {
	storage Storage
	node    etcd.Node
}

// Unlock will attempt to remove the lock held on the mutex
// in the Registry.
func (t *TimedResourceMutex) Unlock() error {
	_, err := t.storage.CompareAndDelete(t.node.Key, "", t.node.CreatedIndex)
	if err != nil {
		err = fmt.Errorf("Received error while unlocking mutex: %v", err)
		log.Error(err)
		return err
	}
	return nil
}
