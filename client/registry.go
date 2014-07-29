package client

import (
	"net/http"
	"time"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/registry"
)

func NewRegistryClient(trans *http.Transport, endpoint, keyPrefix string, requestTimeout time.Duration) (API, error) {
	machines := []string{endpoint}
	client, err := etcd.NewClient(machines, *trans, requestTimeout)
	if err != nil {
		return nil, err
	}

	return registry.New(client, keyPrefix), nil
}
