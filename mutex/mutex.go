package mutex

import (
	"fmt"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

func NewTimedResourceMutex(etcd *etcd.Client, node etcd.Node) *TimedResourceMutex {
	return &TimedResourceMutex{etcd, node}
}

// TimedResourceMutex is a proxy to an auto-expiring mutex
// stored in the Registry. It assumes the mutex creator has
// initialized a timer.
type TimedResourceMutex struct {
	etcd *etcd.Client
	node etcd.Node
}

// Unlock will attempt to remove the lock held on the mutex
// in the Registry.
func (t *TimedResourceMutex) Unlock() error {
	_, err := t.etcd.CompareAndDelete(t.node.Key, "", t.node.CreatedIndex)
	if err != nil {
		err = fmt.Errorf("Received error while unlocking mutex: %v", err)
		log.V(2).Info(err)
		return err
	}
	return nil
}
