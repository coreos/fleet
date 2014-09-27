package registry

import (
	"path"
	"time"

	"github.com/coreos/fleet/etcd"
)

const (
	leasePrefix = "lease"
)

func (r *EtcdRegistry) AcquireLease(name, machID string, period time.Duration) (Lease, error) {
	key := path.Join(r.keyPrefix, leasePrefix, name)
	req := etcd.Create{
		Key:   key,
		Value: machID,
		TTL:   period,
	}

	var lease Lease
	resp, err := r.etcd.Do(&req)
	if err == nil {
		lease = &etcdLease{
			key:   key,
			value: machID,
			idx:   resp.Node.ModifiedIndex,
			etcd:  r.etcd,
		}
	} else if isNodeExist(err) {
		err = nil
	}

	return lease, err
}

// etcdLease implements the Lease interface
type etcdLease struct {
	key   string
	value string
	idx   uint64
	etcd  etcd.Client
}

func (l *etcdLease) Release() error {
	req := etcd.Delete{
		Key:           l.key,
		PreviousIndex: l.idx,
	}
	_, err := l.etcd.Do(&req)
	return err
}

func (l *etcdLease) Renew(period time.Duration) error {
	req := etcd.Set{
		Key:           l.key,
		Value:         l.value,
		PreviousIndex: l.idx,
		TTL:           period,
	}

	resp, err := l.etcd.Do(&req)
	if err == nil {
		l.idx = resp.Node.ModifiedIndex
	}

	return err
}
