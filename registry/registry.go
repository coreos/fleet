package registry

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/version"
)

const (
	keyPrefix = "/_coreos.com/fleet/"
)

type Registry struct {
	etcd	   *etcd.Client
	negotiator *version.Negotiator
}

func New(client *etcd.Client, neg *version.Negotiator) (registry *Registry) {
	return &Registry{client, neg}
}

func (r *Registry) Version() int {
	ver, err := r.negotiator.GetCurrentVersion()
	if err != nil {
		log.V(1).Infof("%v", err)
	}
	return ver
}

func (r *Registry) GetDebugInfo() (string, error) {
	resp, err :=  r.etcd.RawGet(keyPrefix, true, true)
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
