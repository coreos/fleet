package platform

type Cluster interface {
	Create(int) error
	DestroyAll() error
}
