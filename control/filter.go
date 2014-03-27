package control

import (
	"path"
	"sort"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
)

// both arguments must be sorted, result is sorted too
// operation is linear
func intersect(a, b []string) []string {
	ia := 0
	ib := 0

	var r []string

	for {
		if ia == len(a) {
			break
		}
		if ib == len(b) {
			break
		}

		ha := a[ia]
		hb := b[ib]

		if ha == hb {
			r = append(r, ha)
			ia++
			ib++
		} else if ha < hb {
			ia++
		} else {
			ib++
		}
	}
	return r
}

// Finds hosts running all the specified jobNames. Queries the jobs2hosts map to find hosts
// running particular jobs. The values slice in the jobs2hosts map needs to be sorted.
// Returned slice of hosts is sorted.
func hostsRunningAllJobs(jobNames []string, jobs2hosts map[string][]string) []string {
	// loop invariant: hids is sorted
	// at the end of this loop we have hids sorted and having host ids
	// of hosts that run all jobs specified in jobNames
	var hids []string
	for _, jname := range jobNames {
		hid := jobs2hosts[jname]

		if hids == nil {
			hids = hid
		} else {
			// we need hosts that run all depending jobs, hence intersection
			hids = intersect(hids, hid)
		}
	}
	return hids
}

func conflicts(cps []string, js []string) bool {
outer:
	for _, pattern := range cps {
		for _, jobName := range js {
			matched, err := path.Match(pattern, jobName)
			if err != nil {
				log.Errorf("ConflictsWith pattern malformed: %s, error %v", pattern, err)
				continue outer
			}
			if matched {
				return true
			}
		}
	}
	return false
}

// Finds hosts running only jobs not in conflict with specified conflict patterns cps.
// Queries the hosts2jobs map for conflict resolution.
func hostsNotInConflictWith(cps []string, hosts2jobs map[string][]string, flhs []candHost) []candHost {
	var hs []candHost

	for _, ch := range flhs {
		if !conflicts(cps, hosts2jobs[ch.host]) {
			hs = append(hs, ch)
		}
	}
	return hs
}

type candHostByHostID []candHost

func (a candHostByHostID) Len() int           { return len(a) }
func (a candHostByHostID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a candHostByHostID) Less(i, j int) bool { return a[i].host < a[j].host }

func search(flhs []candHost, host string) (int, bool) {
	k := sort.Search(len(flhs), func(i int) bool {
		return flhs[i].host >= host
	})

	if k == len(flhs) || flhs[k].host != host {
		return 0, false
	}
	return k, true
}

// Filters the specified candidates according to clauses in specified job spec.
// Clauses are RequiresHost, DependsOn and ConflictsWith. The returned slice of
// hosts satisfies all three clauses.
func (clus *cluster) filterCandidates(lhs []candHost, spec *JobSpec) ([]candHost, error) {
	// If DependsOn or ConflictsWith clauses are in the job spec
	// then we fetch jobs data from etcd to solve these clauses.
	// This happens once per job and not for every host and is necessary
	// because clus only keeps total load stats for machines, nothing on where jobs are.
	// Alternatively we could cache jobs2hosts and hosts2jobs maps and maintain them.
	var jobs2hosts map[string][]string
	var hosts2jobs map[string][]string
	if len(spec.DependsOn) > 0 || len(spec.ConflictsWith) > 0 {
		jwhs, err := clus.etcd.AllJobs()
		if err != nil {
			return nil, err
		}
		jobs2hosts = make(map[string][]string)
		hosts2jobs = make(map[string][]string)

		for _, jwh := range jwhs {
			hs := jobs2hosts[jwh.Spec.Name]
			jobs2hosts[jwh.Spec.Name] = append(hs, jwh.Host)
			js := hosts2jobs[jwh.Host]
			hosts2jobs[jwh.Host] = append(js, jwh.Spec.Name)
		}
		for _, hs := range jobs2hosts {
			sort.Strings(hs)
		}

		// when one of these clauses is present, we need to search inside lhs afterwards
		// so sort it by host string value here
		sort.Sort(candHostByHostID(lhs))
	}

	flhs := lhs

	if len(spec.RequiresHost) > 0 {
		host := spec.RequiresHost

		k, ok := search(flhs, host)
		if !ok {
			return nil, ErrRequiredHostUnavailable
		}
		flhs = []candHost{flhs[k]}
	}

	if len(spec.DependsOn) > 0 {
		var hs []candHost

		hosts := hostsRunningAllJobs(spec.DependsOn, jobs2hosts)
		for _, host := range hosts {
			k, ok := search(flhs, host)
			if ok {
				hs = append(hs, flhs[k])
			}
		}

		if len(hs) == 0 {
			return nil, ErrDependOnHostUnavailable
		}

		flhs = hs
	}

	// hostsNotInConflictWith doesn't return a sorted list
	// so don't move this clause before the DependsOn clause
	// or make it sorted first
	if len(spec.ConflictsWith) > 0 {

		hs := hostsNotInConflictWith(spec.ConflictsWith, hosts2jobs, flhs)
		if len(hs) == 0 {
			return nil, ErrConflictsWithHostUnavailable
		}

		flhs = hs
	}

	return flhs, nil
}
