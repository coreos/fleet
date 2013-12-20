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
	key := path.Join(keyPrefix, offerPrefix, jo.Job.Name)
	r.etcd.SetDir(key, 0)
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
	key := path.Join(keyPrefix, offerPrefix, jb.JobName, jb.MachineName)
	//TODO: Use a TTL
	r.etcd.Set(key, "", 0)
}

func filterEventJobOffered(resp *etcd.Response) *Event {
	if resp.Action != "set" {
		return nil
	}

	dir, jobName := path.Split(resp.Node.Key)
	prefix := path.Base(strings.TrimSuffix(dir, "/"))

	if prefix != offerPrefix {
		return nil
	}

	return &Event{"EventJobOffered", jobName, nil}
}

func filterEventJobBidSubmitted(resp *etcd.Response) *Event {
	if resp.Action != "set" {
		return nil
	}

	dir, machName := path.Split(resp.Node.Key)
	dir, jobName := path.Split(strings.TrimSuffix(dir, "/"))
	prefix := path.Base(strings.TrimSuffix(dir, "/"))

	if prefix != offerPrefix {
		return nil
	}

	jb := job.NewBid(jobName, machName)
	return &Event{"EventJobBidSubmitted", *jb, nil}
}
