package systemd

type SystemdUnit interface {
	Name() string

	// The first three strings correspond to the LoadState,
	// ActiveState and SubState of a unit.
	State() (string, string, string, []string, error)
}
