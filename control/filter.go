package control

func (clus *cluster) filterCandidates(lhs []candHost, user UserID, spec *JobSpec) ([]candHost, error) {
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

		for _, h := range flhs {
			// TODO(uwedeportivo): check that h has all dependencies running
			hs = append(hs, h)
		}

		if len(hs) == 0 {
			return nil, ErrDependOnHostUnavailable
		}

		flhs = hs
	}

	if len(spec.ConflictsWith) > 0 {
		var hs []candHost

		for _, h := range flhs {
			// TODO(uwedeportivo): check that h has no conflicts
			hs = append(hs, h)
		}

		if len(hs) == 0 {
			return nil, ErrConflictsWithHostUnavailable
		}

		flhs = hs
	}

	return flhs, nil
}
