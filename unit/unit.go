package unit

type SystemdUnit interface {
	Name() string
	Payload() (string, error)

	// The first three strings correspond to the LoadState,
	// ActiveState and SubState of a unit.
	State() (string, string, string, []string, error)
}
