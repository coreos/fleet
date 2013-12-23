package registry

import (
	"path"
	"time"

	log "github.com/golang/glog"
)

const (
	leadershipPrefix = "/leadership/"
)

// Attempt to acquire leadership of a given item. Return boolean success value.
// This is not a fair locking mechanism. It does not queue up multiple leaders.
func (r *Registry) acquireLeadership(item string, machine string, ttl time.Duration) bool {
	log.V(2).Infof("Machine(%s) is attempting to acquire leadership of Item(%s)", machine, item)
	key := path.Join(keyPrefix, leadershipPrefix, item)
	_, err := r.etcd.Create(key, machine, uint64(ttl.Seconds()))
	if err == nil {
		log.V(2).Infof("Machine(%s) successfully acquired leadership of Item(%s)", machine, item)
		return true
	} else {
		log.V(2).Infof("Machine(%s) failed to acquire leadership of Item(%s)", machine, item)
		return false
	}
}
