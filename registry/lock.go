package registry

import (
	"time"
)

const (
	lockPrefix     = "/locks/"
)

// Attempt to acquire a lock in etcd on an arbitrary string. Returns true if
// successful, otherwise false.
func (r *Registry) acquireLock(key string, context string, ttl time.Duration) bool {
	resp, err := r.etcd.Get(key, false, true)

	//FIXME: Here lies a race condition!

	if resp != nil {
		if resp.Node.Value == context {
			_, err = r.etcd.Update(key, context, uint64(ttl.Seconds()))
			return err == nil
		}
	}

	_, err = r.etcd.Create(key, context, uint64(ttl.Seconds()))
	return err == nil
}
