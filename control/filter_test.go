package control

import (
	"strings"
	"testing"
)

func toS(a []string) string {
	return "[" + strings.Join(a, ", ") + "]"
}

func equals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, s := range a {
		if s != b[i] {
			return false
		}
	}
	return true
}

func testIntersect(a, b, expected []string, t *testing.T) {
	anb := intersect(a, b)

	if !equals(anb, expected) {
		t.Errorf("intersection of %s and %s: expected %s, got %s", toS(a), toS(b), toS(expected), toS(anb))
	}
}

func TestIntersect(t *testing.T) {
	testIntersect([]string{"a", "b", "d", "e", "m"}, []string{"b", "c", "f", "m", "n"}, []string{"b", "m"}, t)
	testIntersect([]string{"a", "d"}, []string{"b", "c", "f"}, nil, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a"}, []string{"a"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"b"}, []string{"b"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"c"}, []string{"c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"b", "c"}, []string{"b", "c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "b"}, []string{"a", "b"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "c"}, []string{"a", "c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "b", "c"}, []string{"a", "b", "c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "f"}, []string{"a"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"b", "f"}, []string{"b"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"c", "f"}, []string{"c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"b", "c", "f"}, []string{"b", "c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "b", "f"}, []string{"a", "b"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "c", "f"}, []string{"a", "c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "b", "c", "f"}, []string{"a", "b", "c"}, t)
	testIntersect([]string{"a", "b", "c"}, []string{"a", "f", "g", "h", "i"}, []string{"a"}, t)
}

func testHostsRunningAllJobs(clus *cluster, jobNames []string, expected []string, t *testing.T) {
	hosts, err := clus.hostsRunningAllJobs(jobNames)
	if err != nil {
		t.Fatalf("couldn't execute hostsRunningAllJobs: %v", err)
	}

	if !equals(hosts, expected) {
		t.Errorf("hostsRunningAllJobs: %s, expected %s, got %s", toS(jobNames), toS(expected), toS(hosts))
	}
}

func TestHostsRunningAllJobs(t *testing.T) {
	record := make(map[string]string)

	etcd := &mockEtcd{
		record: record,
	}

	etcd.declareHost("host1")
	etcd.declareHost("host2")
	etcd.declareHost("host3")
	etcd.declareHost("host4")
	etcd.declareHost("host5")
	etcd.declareHost("host6")

	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host1")
	etcd.declareJob(newTestJob(2, 100, 1024, 10), "host1")
	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host2")
	etcd.declareJob(newTestJob(2, 100, 1024, 10), "host2")
	etcd.declareJob(newTestJob(3, 100, 1024, 10), "host3")
	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host3")
	etcd.declareJob(newTestJob(3, 100, 1024, 10), "host6")
	etcd.declareJob(newTestJob(4, 100, 1024, 10), "host6")
	etcd.declareJob(newTestJob(4, 100, 1024, 10), "host4")
	etcd.declareJob(newTestJob(4, 100, 1024, 10), "host1")

	ctrl, err := NewJobControl(etcd, new(mockUniformMachineDB))
	if err != nil {
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	testHostsRunningAllJobs(clus, []string{"job1"}, []string{"host1", "host2", "host3"}, t)
	testHostsRunningAllJobs(clus, []string{"job1", "job2"}, []string{"host1", "host2"}, t)
	testHostsRunningAllJobs(clus, []string{"job1", "job3"}, []string{"host3"}, t)
	testHostsRunningAllJobs(clus, []string{"job2", "job3"}, nil, t)
	testHostsRunningAllJobs(clus, []string{"job3"}, []string{"host3", "host6"}, t)
	testHostsRunningAllJobs(clus, []string{"job4"}, []string{"host1", "host4", "host6"}, t)
	testHostsRunningAllJobs(clus, []string{"job2", "job4"}, []string{"host1"}, t)
	testHostsRunningAllJobs(clus, []string{"job1", "job4"}, []string{"host1"}, t)
	testHostsRunningAllJobs(clus, []string{"job3", "job4"}, []string{"host6"}, t)
}
