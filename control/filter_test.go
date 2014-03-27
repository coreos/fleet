package control

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func toS(a []string) string {
	return "[" + strings.Join(a, ", ") + "]"
}

func ctoS(a []candHost) string {
	ss := make([]string, len(a))
	for i, v := range a {
		ss[i] = v.host
	}
	return toS(ss)
}

func stringSlicesEqual(a, b []string) bool {
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

func candHostSlicesEqual(a, b []candHost) bool {
	if len(a) != len(b) {
		return false
	}

	for i, s := range a {
		if s.host != b[i].host {
			return false
		}
	}
	return true
}

func testIntersect(a, b, expected []string, t *testing.T) {
	anb := intersect(a, b)

	if !stringSlicesEqual(anb, expected) {
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

func testHostsRunningAllJobs(jobs2hosts map[string][]string, jobNames []string, expected []string, t *testing.T) {
	hosts := hostsRunningAllJobs(jobNames, jobs2hosts)

	if !stringSlicesEqual(hosts, expected) {
		t.Errorf("hostsRunningAllJobs: %s, expected %s, got %s", toS(jobNames), toS(expected), toS(hosts))
	}
}

func declareJob(jobs2hosts map[string][]string, job, host int) {
	jobName := fmt.Sprintf("job%d", job)
	hostName := fmt.Sprintf("host%d", host)

	jobs2hosts[jobName] = append(jobs2hosts[jobName], hostName)
}

func TestHostsRunningAllJobs(t *testing.T) {
	jobs2hosts := make(map[string][]string)

	declareJob(jobs2hosts, 1, 1)
	declareJob(jobs2hosts, 2, 1)
	declareJob(jobs2hosts, 1, 2)
	declareJob(jobs2hosts, 2, 2)
	declareJob(jobs2hosts, 3, 3)
	declareJob(jobs2hosts, 1, 3)
	declareJob(jobs2hosts, 3, 6)
	declareJob(jobs2hosts, 4, 6)
	declareJob(jobs2hosts, 4, 4)
	declareJob(jobs2hosts, 4, 1)

	for _, hs := range jobs2hosts {
		sort.Strings(hs)
	}

	testHostsRunningAllJobs(jobs2hosts, []string{"job1"}, []string{"host1", "host2", "host3"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job1", "job2"}, []string{"host1", "host2"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job1", "job3"}, []string{"host3"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job2", "job3"}, nil, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job3"}, []string{"host3", "host6"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job4"}, []string{"host1", "host4", "host6"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job2", "job4"}, []string{"host1"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job1", "job4"}, []string{"host1"}, t)
	testHostsRunningAllJobs(jobs2hosts, []string{"job3", "job4"}, []string{"host6"}, t)
}

func testHostsNotInConflictWith(hosts2jobs map[string][]string, conflictPatterns []string,
	chs []candHost, expected []candHost, t *testing.T) {
	hosts := hostsNotInConflictWith(conflictPatterns, hosts2jobs, chs)

	if !candHostSlicesEqual(hosts, expected) {
		t.Errorf("hostsNotInConflictWith: %s, expected %s, got %s", toS(conflictPatterns), ctoS(expected), ctoS(hosts))
	}
}

func declareHost(hosts2jobs map[string][]string, jobName string, host int) {
	hostName := fmt.Sprintf("host%d", host)

	hosts2jobs[hostName] = append(hosts2jobs[hostName], jobName)
}

func candidateHosts(hids []string) []candHost {
	chs := make([]candHost, len(hids))

	for i, h := range hids {
		chs[i].host = h
	}
	return chs
}

func TestHostsNotInConflictWith(t *testing.T) {
	hosts2jobs := make(map[string][]string)

	declareHost(hosts2jobs, "foo.db.aux", 1)
	declareHost(hosts2jobs, "foo.mysql.main", 1)
	declareHost(hosts2jobs, "foo.engine.service", 2)
	declareHost(hosts2jobs, "bar.ruby.rails", 3)
	declareHost(hosts2jobs, "foo.rabbitmq.main", 4)
	declareHost(hosts2jobs, "foo.rabbitmq.aux", 5)
	declareHost(hosts2jobs, "foo.bar.discover", 1)
	declareHost(hosts2jobs, "gus.happy.feet", 4)
	declareHost(hosts2jobs, "tinker.tailor.soldier.spy", 6)
	declareHost(hosts2jobs, "tinker.control.main", 7)
	declareHost(hosts2jobs, "eng.spy", 7)
	declareHost(hosts2jobs, "range.soldier.hoop", 7)

	testHostsNotInConflictWith(hosts2jobs, []string{"foo*"},
		candidateHosts([]string{"host1", "host2", "host3"}),
		candidateHosts([]string{"host3"}), t)

	testHostsNotInConflictWith(hosts2jobs, []string{"foo*"},
		candidateHosts([]string{"host8"}),
		candidateHosts([]string{"host8"}), t)

	testHostsNotInConflictWith(hosts2jobs, []string{"foo*"},
		candidateHosts([]string{"host3", "host4", "host6"}),
		candidateHosts([]string{"host3", "host6"}), t)

	testHostsNotInConflictWith(hosts2jobs, []string{"*bar*", "tinker*"},
		candidateHosts([]string{"host1", "host2", "host3", "host4", "host5", "host6", "host7", "host8"}),
		candidateHosts([]string{"host2", "host4", "host5", "host8"}), t)

	testHostsNotInConflictWith(hosts2jobs, []string{"*happy*", "*foo*", "*spy"},
		candidateHosts([]string{"host1", "host2", "host3", "host4", "host5", "host6", "host7", "host8"}),
		candidateHosts([]string{"host3", "host8"}), t)

}

func newFilterTestJob(name string, hostRequired string, dependsOn []string, conflictsWith []string) *JobSpec {
	return &JobSpec{
		Name:          name,
		RequiresHost:  hostRequired,
		DependsOn:     dependsOn,
		ConflictsWith: conflictsWith,
	}
}

func testFilterCandidates(clus *cluster, chs []candHost, spec *JobSpec, expected []candHost, expectedError error, t *testing.T) {
	hosts, err := clus.filterCandidates(chs, spec)
	if err != nil && err != expectedError {
		t.Fatalf("filterCandidates failed: %v", err)
	}

	if !candHostSlicesEqual(hosts, expected) {
		t.Errorf("filterCandidates: %s, expected %s, got %s", spec.Name, ctoS(expected), ctoS(hosts))
	}
}

func TestFilterCandidates(t *testing.T) {
	record := make(map[string]string)

	etcd := &mockEtcd{
		record: record,
	}

	etcd.declareHost("host1")
	etcd.declareHost("host2")
	etcd.declareHost("host3")
	etcd.declareHost("host4")

	etcd.declareJob(newFilterTestJob("mysql.main", "", nil, nil), "host1")
	etcd.declareJob(newFilterTestJob("logging.service", "", nil, nil), "host1")
	etcd.declareJob(newFilterTestJob("logging.service", "", nil, nil), "host2")

	ctrl, err := NewJobControl(etcd, new(mockUniformMachineDB))
	if err != nil {
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	testFilterCandidates(clus, candidateHosts([]string{"host1", "host2", "host3", "host4"}),
		newFilterTestJob("test1", "host1", nil, nil), candidateHosts([]string{"host1"}), nil, t)

	testFilterCandidates(clus, candidateHosts([]string{"host2", "host3", "host4"}),
		newFilterTestJob("test1", "host1", nil, nil), nil, ErrRequiredHostUnavailable, t)

	testFilterCandidates(clus, candidateHosts([]string{"host1", "host2", "host3", "host4"}),
		newFilterTestJob("test1", "", []string{"mysql.main", "logging.service"}, nil), candidateHosts([]string{"host1"}), nil, t)

	testFilterCandidates(clus, candidateHosts([]string{"host1", "host2", "host3", "host4"}),
		newFilterTestJob("test1", "", []string{"unicorns"}, nil), nil, ErrDependOnHostUnavailable, t)

	testFilterCandidates(clus, candidateHosts([]string{"host1", "host2", "host3", "host4"}),
		newFilterTestJob("test1", "", []string{"logging.service"}, nil), candidateHosts([]string{"host1", "host2"}), nil, t)

	testFilterCandidates(clus, candidateHosts([]string{"host1", "host2", "host3", "host4"}),
		newFilterTestJob("test1", "", nil, []string{"mysql.main"}), candidateHosts([]string{"host2", "host3", "host4"}), nil, t)

	testFilterCandidates(clus, candidateHosts([]string{"host1", "host2", "host3", "host4"}),
		newFilterTestJob("test1", "", []string{"logging.service"}, []string{"mysql.main"}), candidateHosts([]string{"host2"}), nil, t)
}
