package pkg

import (
	"sync"
)

type Set interface {
	Add(string)
	Remove(string)
	Contains(string) bool
	Length() int
	Values() []string
	Copy() Set
	Sub(Set) Set
}

func NewUnsafeSet() *unsafeSet {
	return &unsafeSet{make(map[string]struct{})}
}

func NewThreadsafeSet() *tsafeSet {
	us := NewUnsafeSet()
	return &tsafeSet{us, sync.RWMutex{}}
}

type unsafeSet struct {
	d map[string]struct{}
}

func (us *unsafeSet) Add(value string) {
	us.d[value] = struct{}{}
}

func (us *unsafeSet) Remove(value string) {
	delete(us.d, value)
}

func (us *unsafeSet) Contains(value string) (exists bool) {
	_, exists = us.d[value]
	return
}

func (us *unsafeSet) Length() int {
	return len(us.d)
}

func (us *unsafeSet) Values() (values []string) {
	values = make([]string, 0)
	for val, _ := range us.d {
		values = append(values, val)
	}
	return
}

func (us *unsafeSet) Copy() Set {
	cp := NewUnsafeSet()
	for val, _ := range us.d {
		cp.Add(val)
	}

	return cp
}

func (us *unsafeSet) Sub(other Set) Set {
	oValues := other.Values()
	result := us.Copy().(*unsafeSet)

	for _, val := range oValues {
		if _, ok := result.d[val]; !ok {
			continue
		}
		delete(result.d, val)
	}

	return result
}

type tsafeSet struct {
	us *unsafeSet
	m  sync.RWMutex
}

func (ts *tsafeSet) Add(value string) {
	ts.m.Lock()
	defer ts.m.Unlock()
	ts.us.Add(value)
}

func (ts *tsafeSet) Remove(value string) {
	ts.m.Lock()
	defer ts.m.Unlock()
	ts.us.Remove(value)
}

func (ts *tsafeSet) Contains(value string) (exists bool) {
	ts.m.RLock()
	defer ts.m.RUnlock()
	return ts.us.Contains(value)
}

func (ts *tsafeSet) Length() int {
	ts.m.RLock()
	defer ts.m.RUnlock()
	return ts.us.Length()
}

func (ts *tsafeSet) Values() (values []string) {
	ts.m.RLock()
	defer ts.m.RUnlock()
	return ts.us.Values()
}

func (ts *tsafeSet) Copy() Set {
	ts.m.RLock()
	defer ts.m.RUnlock()
	usResult := ts.us.Copy().(*unsafeSet)
	return &tsafeSet{usResult, sync.RWMutex{}}
}

func (ts *tsafeSet) Sub(other Set) Set {
	ts.m.RLock()
	defer ts.m.RUnlock()
	usResult := ts.us.Sub(other).(*unsafeSet)
	return &tsafeSet{usResult, sync.RWMutex{}}
}
