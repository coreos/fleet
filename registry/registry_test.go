package registry

import (
	"testing"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

func NewTestRegistry() Registry {
	m := &MemoryStorage{}
	return &FleetRegistry{m, ""}
}

func TestCreateJob(t *testing.T) {
	r := NewTestRegistry()

	j := job.NewJob("foo", *unit.NewUnit("bar"))
	err := r.CreateJob(j)
	if err != nil {
		t.Fatalf("CreateJob failed with err: %v", err)
	}
}

// MemoryStorage is a dumb in-memory implementation of Storage for testing
type MemoryStorage struct{}

// TODO(jonboulle): implement these methods.
func (m *MemoryStorage) CompareAndDelete(key string, prevValue string, prevIndex uint64) (*etcd.Response, error) {
	return nil, nil
}

func (m *MemoryStorage) Create(key string, value string, ttl uint64) (*etcd.Response, error) {
	return nil, nil
}
func (m *MemoryStorage) Delete(key string, recursive bool) (*etcd.Response, error) {
	return nil, nil
}
func (m *MemoryStorage) Get(key string, sort, recursive bool) (*etcd.Response, error) {
	return nil, nil
}
func (m *MemoryStorage) RawGet(key string, sort, recursive bool) (*etcd.RawResponse, error) {
	return nil, nil
}

func (m *MemoryStorage) Set(key string, value string, ttl uint64) (*etcd.Response, error) {
	return nil, nil
}

func (m *MemoryStorage) Update(key string, value string, ttl uint64) (*etcd.Response, error) {
	return nil, nil
}
