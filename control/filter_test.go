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

func testHostsRunningAllJobs(jobs2hosts map[string][]string, jobNames []string, expected []string, t *testing.T) {
	hosts := hostsRunningAllJobs(jobNames, jobs2hosts)

	if !equals(hosts, expected) {
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
