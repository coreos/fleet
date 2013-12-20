package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix      = "/coreos.com/coreinit/"
	lockPrefix     = "/locks/"
	machinePrefix  = "/machines/"
	requestPrefix  = "/request/"
	schedulePrefix = "/schedule/"
	statePrefix    = "/state/"
)

type Registry struct {
	etcd *etcd.Client
}

func New(client *etcd.Client) (registry *Registry) {
	return &Registry{client}
}

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

func marshal(obj interface{}) (string, error) {
	encoded, err := json.Marshal(obj)
	if err == nil {
		return string(encoded), nil
	} else {
		return "", errors.New(fmt.Sprintf("Unable to JSON-serialize object: %s", err))
	}
}

func unmarshal(val string, obj interface{}) error {
	err := json.Unmarshal([]byte(val), &obj)
	if err == nil {
		return nil
	} else {
		return errors.New(fmt.Sprintf("Unable to JSON-deserialize object: %s", err))
	}
}
