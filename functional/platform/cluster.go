package platform

type Cluster interface {
	CreateMultiple(int, MachineConfig) error
	DestroyAll() error
}

// MachineConfig defines the parameters that should
// be considered when creating a new cluster member.
type MachineConfig struct {
}
