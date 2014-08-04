package engine

// Reconciler attempts to advance the state of Jobs and JobOffers
// towards their desired states wherever discrepancies are identified.
type Reconciler interface {
	Reconcile(*Engine, chan struct{})
}
