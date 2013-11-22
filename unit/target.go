package unit

type SystemdTarget struct {
	name string
}

func NewSystemdTarget(name string) *SystemdTarget {
	return &SystemdTarget{name}
}

func (st *SystemdTarget) Name() string {
	return st.name
}

func (st *SystemdTarget) State() (string, []string, error) {
	return "active", []string{}, nil
}

func (st *SystemdTarget) Payload() (string, error) {
	return "", nil
}
