package registry

import (
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/event"
	"github.com/coreos/fleet/mutex"
	"github.com/coreos/fleet/version"
)

const (
	versionPrefix = "version"
	currentVersionKey = "current"
	negotiatorPrefix = "negotiators"
)

func (r *Registry) GetVersionNegotiators() (negotiators []version.Negotiator, err error) {
	// Seed the map from the list of machines to handle fleet machines from
	// a time before versioning negotiation
	negMap := make(map[string]version.Negotiator)
	for _, m := range r.GetActiveMachines() {
		n, err := version.NewNegotiator(m.BootId, 0, 0)
		if err == nil {
			negMap[n.Name] = *n
		} else {
			log.Errorf("Failed building version.Negotiator from registry: %v", err)
		}
	}

	key := path.Join(keyPrefix, versionPrefix, negotiatorPrefix)
	resp, err := r.etcd.Get(key, false, true)

	if err != nil {
		return
	}

	for _, node := range resp.Node.Nodes {
		var n version.Negotiator
		err := unmarshal(node.Value, &n)

		if err != nil {
			log.Error(err.Error())
			continue
		}

		negMap[n.Name] = n
	}

	negotiators = make([]version.Negotiator, 0)
	for _, n := range negMap {
		negotiators = append(negotiators, n)
	}

	return
}

func (r *Registry) GetClusterVersion() (ver int, err error) {
	key := path.Join(keyPrefix, versionPrefix, currentVersionKey)
	resp, err := r.etcd.Get(key, false, false)

	// Assume an error is KeyNotFound for now
	if err != nil {
		ver = version.VersionUninitialized
		err = nil
	} else {
		ver, err = strconv.Atoi(resp.Node.Value)
		if err != nil {
			ver = version.VersionUninitialized
		}
	}

	return
}

func (r *Registry) SetClusterVersion(version int) error {
	key := path.Join(keyPrefix, versionPrefix, currentVersionKey)
	_, err := r.etcd.Set(key, strconv.Itoa(version), 0)
	return err
}

func (r *Registry) SetVersionNegotiator(n *version.Negotiator, ttl time.Duration) (err error) {
	json, err := marshal(n)
	if err != nil {
		return
	}

	key := path.Join(keyPrefix, versionPrefix, negotiatorPrefix, n.Name)
	_, err = r.etcd.Set(key, json, uint64(ttl.Seconds()))

	return
}

func (r *Registry) DestroyVersionNegotiator(n *version.Negotiator) (err error) {
	key := path.Join(keyPrefix, negotiatorPrefix, n.Name)
	_, err = r.etcd.Delete(key, false)
	return
}

func (r *Registry) LockClusterVersion(context string) *mutex.TimedResourceMutex {
	return r.lockResource("cluster", "version", context)
}


func NewClusterState(reg *Registry) *ClusterState {
	return &ClusterState{reg}
}

type ClusterState struct {
	registry *Registry
}

func newClusterState(reg *Registry) *ClusterState {
	return &ClusterState{registry: reg}
}

func (self *ClusterState) Version() (int, bool, error) {
	ver, err := self.registry.GetClusterVersion()
	return ver, err != nil, err
}

func (self *ClusterState) Upgrade(version int) error {
	return self.registry.SetClusterVersion(version)
}

func (self *ClusterState) Publish(n *version.Negotiator, ttl time.Duration) error {
	return self.registry.SetVersionNegotiator(n, ttl)
}

func (self *ClusterState) Unpublish(n *version.Negotiator) error {
	return self.registry.DestroyVersionNegotiator(n)
}

func (self *ClusterState) AcquireMutex(n *version.Negotiator) *mutex.TimedResourceMutex {
	return self.registry.LockClusterVersion(n.Name)
}

func (self ClusterState) Negotiators() ([]version.Negotiator, error) {
	return self.registry.GetVersionNegotiators()
}

func filterEventNegotiatorPublished(resp *etcd.Response) *event.Event {
	if resp.Action != "set" {
		return nil
	}

	dir := path.Dir(resp.Node.Key)
	dir = strings.TrimSuffix(dir, "/")
	dir, prefixName := path.Split(dir)

	if prefixName != negotiatorPrefix {
		return nil
	}

	var negotiator version.Negotiator
	err := unmarshal(resp.Node.Value, &negotiator)
	if err != nil {
		log.Errorf("Failed unmarshaling version negotiator: %v", err)
		return nil
	}

	return &event.Event{"EventNegotiatorPublished", negotiator, nil}
}

func filterEventNegotiatorRemoved(resp *etcd.Response) *event.Event {
	if resp.Action != "delete" || resp.Action != "expire" {
		return nil
	}

	dir := path.Dir(resp.Node.Key)
	dir = strings.TrimSuffix(dir, "/")
	dir, prefixName := path.Split(dir)

	if prefixName != negotiatorPrefix {
		return nil
	}

	var negotiator version.Negotiator
	err := unmarshal(resp.Node.Value, &negotiator)
	if err != nil {
		log.Errorf("Failed unmarshaling version negotiator: %v", err)
		return nil
	}

	return &event.Event{"EventNegotiatorRemoved", negotiator, nil}
}

func filterEventClusterUpgraded(resp *etcd.Response) *event.Event {
	if resp.Action != "set" {
		return nil
	}

	if path.Base(resp.Node.Key) != currentVersionKey {
		return nil
	}

	ver, err := strconv.Atoi(resp.Node.Value)
	if err != nil {
		log.Errorf("Failed unmarshaling version: %v", err)
		return nil
	}

	return &event.Event{"EventClusterUpgraded", ver, nil}
}
