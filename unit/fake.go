package unit

import (
	"sync"

	"github.com/coreos/fleet/pkg"
)

func NewFakeUnitManager() *FakeUnitManager {
	return &FakeUnitManager{u: map[string]bool{}}
}

type FakeUnitManager struct {
	sync.RWMutex
	u map[string]bool
}

func (fum *FakeUnitManager) Load(name string, u Unit) error {
	fum.Lock()
	defer fum.Unlock()

	fum.u[name] = false
	return nil
}

func (fum *FakeUnitManager) Unload(name string) {
	fum.Lock()
	defer fum.Unlock()

	delete(fum.u, name)
}

func (fum *FakeUnitManager) Start(string) {}
func (fum *FakeUnitManager) Stop(string)  {}

func (fum *FakeUnitManager) Units() ([]string, error) {
	fum.RLock()
	defer fum.RUnlock()

	lst := make([]string, 0, len(fum.u))
	for name, _ := range fum.u {
		lst = append(lst, name)
	}
	return lst, nil
}

func (fum *FakeUnitManager) GetUnitState(name string) (us *UnitState, err error) {
	fum.RLock()
	defer fum.RUnlock()

	if _, ok := fum.u[name]; ok {
		us = &UnitState{"loaded", "active", "running", ""}
	}
	return
}

func (fum *FakeUnitManager) GetUnitStates(filter pkg.Set) (map[string]*UnitState, error) {
	fum.RLock()
	defer fum.RUnlock()

	states := make(map[string]*UnitState)
	for _, name := range filter.Values() {
		if _, ok := fum.u[name]; ok {
			states[name] = &UnitState{"loaded", "active", "running", ""}
		}
	}

	return states, nil
}

func (fum *FakeUnitManager) MarshalJSON() ([]byte, error) {
	return nil, nil
}
