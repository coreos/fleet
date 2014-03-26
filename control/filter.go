package control

import "sort"

func (clus *cluster) passesDependsOnFilter(h candHost, hids []string) bool {
	// hids are sorted coming in

	k := sort.SearchStrings(hids, h.host)
	return k < len(hids) && hids[k] == h.host
}

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

func (clus *cluster) filterCandidates(lhs []candHost, user string, spec *JobSpec) ([]candHost, error) {
	flhs := lhs

	if len(spec.RequiresHost) > 0 {
		var hs []candHost

		for _, h := range flhs {
			if h.host == spec.RequiresHost {
				hs = append(hs, h)
				// only one, we found him
				break
			}
		}

		if len(hs) == 0 {
			return nil, ErrRequiredHostUnavailable
		}

		flhs = hs
	}

	if len(spec.DependsOn) > 0 {
		var hs []candHost

		// loop invariant: hids is sorted
		// at the end of this loop we have hids sorted and having host ids
		// of hosts that run all jobs from spec.DependsOn
		var hids []string
		for _, jname := range spec.DependsOn {
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

		for _, h := range flhs {
			if clus.passesDependsOnFilter(h, hids) {
				hs = append(hs, h)
			}
		}

		if len(hs) == 0 {
			return nil, ErrDependOnHostUnavailable
		}

		flhs = hs
	}

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
