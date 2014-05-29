package etcd

import (
	"net/http"

	goetcd "github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
)

type Client interface {
	GetCluster() []string
	SetTransport(tr *http.Transport)

	CompareAndDelete(key string, prevValue string, prevIndex uint64) (*goetcd.Response, error)
	Create(key string, value string, ttl uint64) (*goetcd.Response, error)
	Delete(key string, recursive bool) (*goetcd.Response, error)
	Get(key string, sort, recursive bool) (*goetcd.Response, error)
	RawGet(key string, sort, recursive bool) (*goetcd.RawResponse, error)
	Set(key string, value string, ttl uint64) (*goetcd.Response, error)
	Update(key string, value string, ttl uint64) (*goetcd.Response, error)

	Watch(prefix string, waitIndex uint64, recursive bool, receiver chan *goetcd.Response, stop chan bool) (*goetcd.Response, error)
}

func NewClient(servers []string) Client {
	c := goetcd.NewClient(servers)
	c.SetConsistency(goetcd.STRONG_CONSISTENCY)
	return c
}
