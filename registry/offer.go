package registry

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-etcd/etcd"
	//log "github.com/golang/glog"

	"github.com/coreos/coreinit/job"
	"github.com/coreos/coreinit/machine"
)

const (
	offerPrefix    = "offer"
)

func (r *Registry) CreateJobOffer(jo *job.JobOffer) {
	key := path.Join(keyPrefix, offerPrefix, jo.Job.Name, "object")
	json, _ := marshal(jo)
	r.etcd.Set(key, json, 0)
}

func (r *Registry) ClaimJobOffer(jobName string, m *machine.Machine, ttl time.Duration) bool {
	key := path.Join(keyPrefix, offerPrefix, jobName)
	_, err := r.etcd.Get(key, false, true)
	if err != nil {
		return false
	}

	key = path.Join(keyPrefix, lockPrefix, fmt.Sprintf("offer-%s", jobName))
	return r.acquireLock(key, m.BootId, ttl)
}

func (r *Registry) ResolveJobOffer(jobName string) {
	key := path.Join(keyPrefix, offerPrefix, jobName)
	r.etcd.Delete(key, true)
}

func (r *Registry) SubmitJobBid(jb *job.JobBid) {
	key := path.Join(keyPrefix, offerPrefix, jb.JobName, "bids", jb.MachineName)
	//TODO: Use a TTL
	r.etcd.Set(key, "", 0)
}

func (self *EventStream) filterEventJobOffered(resp *etcd.Response) *Event {
	if resp.Action != "set" {
		return nil
	}

	dir, base := path.Split(resp.Node.Key)

	if base != "object" {
		return nil
	}

	dir = path.Dir(strings.TrimSuffix(dir, "/"))
	prefix := path.Base(strings.TrimSuffix(dir, "/"))

	if prefix != offerPrefix {
		return nil
	}

	var jo job.JobOffer
	//TODO: handle error from unmarshal
	unmarshal(resp.Node.Value, &jo)

	return &Event{"EventJobOffered", jo, nil}
}

func filterEventJobBidSubmitted(resp *etcd.Response) *Event {
	if resp.Action != "set" {
		return nil
	}

	dir, machName := path.Split(resp.Node.Key)
	dir, prefix := path.Split(strings.TrimSuffix(dir, "/"))

	if prefix != "bids" {
		return nil
	}

	dir, jobName := path.Split(strings.TrimSuffix(dir, "/"))
	prefix = path.Base(strings.TrimSuffix(dir, "/"))

	if prefix != offerPrefix {
		return nil
	}

	jb := job.NewBid(jobName, machName)
	return &Event{"EventJobBidSubmitted", *jb, nil}
}
