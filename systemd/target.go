package systemd

type SystemdTarget struct {
	name string
}

func NewSystemdTarget(name string) *SystemdTarget {
	return &SystemdTarget{name}
}

func (st *SystemdTarget) Name() string {
	return st.name
}

func (st *SystemdTarget) State() (string, string, string, []string, error) {
	// This is what systemd will return based on how we use
	// targets today.
	return "masked", "inactive", "dead", []string{}, nil
}
