package unit

type SystemdTarget struct {
	Name string
}

func NewSystemdTarget(name string) *SystemdTarget {
	tgt := SystemdTarget{name}
	tgt.persist()
	return &tgt
}

func (st *SystemdTarget) persist() error {
	return writeUnit(st.Name, "")
}
