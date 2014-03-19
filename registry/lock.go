package registry

import (
	"fmt"
	"path"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/mutex"
)

const (
	// ResourceMutexTTL is the number of seconds to allow a mutex to be held on a resource
	ResourceMutexTTL = 10

	mutexPrefix = "/mutex/"
)

// lockResource will attempt to lock a mutex on a resource defined by the
// provided class and id. The context will be persisted to the Registry to
// track by whom the mutex is currently locked.
func (r *Registry) lockResource(class, id, context string) *mutex.TimedResourceMutex {
	mutexName := fmt.Sprintf("%s-%s", class, id)
	log.V(2).Infof("Attempting to acquire mutex on %s", mutexName)

	key := path.Join(keyPrefix, mutexPrefix, mutexName)
	resp, err := r.etcd.Create(key, context, uint64(ResourceMutexTTL))

	if err != nil {
		log.V(2).Infof("Failed to acquire mutex on %s", mutexName)
		return nil
	}

	log.V(2).Infof("Successfully acquired mutex on %s", mutexName)
	return mutex.NewTimedResourceMutex(r.etcd, *resp.Node)
}


