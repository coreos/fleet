package registry

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
)

const DefaultKeyPrefix = "/_coreos.com/fleet/"

type Registry struct {
	etcd      *etcd.Client
	keyPrefix string
}

func New(client *etcd.Client, keyPrefix string) (registry *Registry) {
	return &Registry{client, keyPrefix}
}

func (r *Registry) GetDebugInfo() (string, error) {
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
