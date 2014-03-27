package control

import "sort"

func (clus *cluster) passesConflictsWithFilter(h candHost, spec *JobSpec) bool {
	// TODO(uwedeportivo): implement
	return false
}

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

func (clus *cluster) hostsRunningAllJobs(jobNames []string) ([]string, error) {
	// loop invariant: hids is sorted
	// at the end of this loop we have hids sorted and having host ids
	// of hosts that run all jobs specified in jobNames
	var hids []string
	for _, jname := range jobNames {
		hid, err := clus.etcd.HostsForJob(jname)

		sort.Strings([]string(hid))
		if err != nil {
			return nil, err
		}
		if hids == nil {
			hids = hid
		} else {
			// we need hosts that run all depending jobs, hence intersection
			hids = intersect(hids, hid)
		}
	}
	return hids, nil
}

func (clus *cluster) filterCandidates(lhs []candHost, user string, spec *JobSpec) ([]candHost, error) {
	flhs := lhs

	if len(spec.ConflictsWith) > 0 {
		var hs []candHost

		for _, h := range flhs {
			if clus.passesConflictsWithFilter(h, spec) {
				hs = append(hs, h)
			}
		}

		if len(hs) == 0 {
			return nil, ErrConflictsWithHostUnavailable
		}

		flhs = hs
	}

	return flhs, nil
}
