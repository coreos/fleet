package registry

import (
	"encoding/json"
	"fmt"

	goetcd "github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"

	"github.com/coreos/fleet/etcd"
)

const DefaultKeyPrefix = "/_coreos.com/fleet/"

// EtcdRegistry fulfils the Registry interface and uses etcd as a backend
type EtcdRegistry struct {
	etcd      etcd.Client
	keyPrefix string
}

// New creates a new EtcdRegistry with the given parameters
func New(client etcd.Client, keyPrefix string) (registry Registry) {
	return &EtcdRegistry{client, keyPrefix}
}

func (r *EtcdRegistry) GetDebugInfo() (string, error) {
	resp, err := r.etcd.RawGet(r.keyPrefix, true, true)
	if err != nil {
		return "", err
	}
	return string(resp.Body), nil
}

func marshal(obj interface{}) (string, error) {
	encoded, err := json.Marshal(obj)
	if err == nil {
		return string(encoded), nil
	}
	return "", fmt.Errorf("unable to JSON-serialize object: %s", err)
}

func unmarshal(val string, obj interface{}) error {
	err := json.Unmarshal([]byte(val), &obj)
	if err == nil {
		return nil
	}
	return fmt.Errorf("unable to JSON-deserialize object: %s", err)
}

func isKeyNotFound(err error) bool {
	e, ok := err.(*goetcd.EtcdError)
	return ok && e.ErrorCode == etcd.EcodeKeyNotFound
}
