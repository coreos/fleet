package registry

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
)

const DefaultKeyPrefix = "/_coreos.com/fleet/"

// FleetRegistry is the standard Registry implementation used by fleet
type FleetRegistry struct {
	storage   Storage
	keyPrefix string
}

// New creates an etcd-backed FleetRegistry with the given parameters
func New(client *etcd.Client, keyPrefix string) (registry Registry) {
	return &FleetRegistry{client, keyPrefix}
}

func (r *FleetRegistry) GetDebugInfo() (string, error) {
	resp, err := r.storage.RawGet(r.keyPrefix, true, true)
	if err != nil {
		return "", err
	}
	return string(resp.Body), nil
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
