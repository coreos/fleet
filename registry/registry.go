// Package registry is the primary object of coreos-registry
package registry

import (
	"github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix = "/coreos.com/coreinit/"
	machinePrefix = "/machines/"
	systemPrefix = "/system/"
	schedulePrefix = "/schedule/"
)

type Registry struct {
	Etcd *etcd.Client
}

func NewRegistry() (registry *Registry) {
	etcdC := etcd.NewClient(nil)
	registry = &Registry{etcdC}
	return registry
}
