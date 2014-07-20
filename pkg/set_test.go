package pkg

import (
	"reflect"
	"testing"
)

func TestUnsafeSet(t *testing.T) {
	driveSetTests(t, NewUnsafeSet())
}

func TestThreadsafeSet(t *testing.T) {
	driveSetTests(t, NewThreadsafeSet())
}

func driveSetTests(t *testing.T, s Set) {
	eValues := []string{}
	values := s.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	// Add three items, ensure they show up
	s.Add("foo")
	s.Add("bar")
	s.Add("baz")

	eValues = []string{"foo", "bar", "baz"}
	values = s.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	for _, v := range eValues {
		if !s.Contains(v) {
			t.Fatalf("Expect s.Contains(%q) to be true, got false", v)
		}
	}

	if l := s.Length(); l != 3 {
		t.Fatalf("Expected length=3, got %d", l)
	}

	// Add the same item a second time, ensuring it is not duplicated
	s.Add("foo")

	values = s.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	// Remove all items, ensure they are gone
	s.Remove("foo")
	s.Remove("bar")
	s.Remove("baz")

	eValues = []string{}
	values = s.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	if l := s.Length(); l != 0 {
		t.Fatalf("Expected length=0, got %d", l)
	}

	// Create a copy, and ensure it is unlinked to the other Set
	s.Add("foo")
	s.Add("bar")
	cp := s.Copy()
	s.Remove("foo")
	cp.Add("baz")

	eValues = []string{"foo", "bar", "baz"}
	values = cp.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	eValues = []string{"bar"}
	values = s.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	// Subtract values from a Set, ensuring a new Set is created and
	// the original Sets are unmodified
	sub := cp.Sub(s)

	eValues = []string{"foo", "bar", "baz"}
	values = cp.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	eValues = []string{"bar"}
	values = s.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}

	eValues = []string{"foo", "baz"}
	values = sub.Values()
	if !reflect.DeepEqual(values, eValues) {
		t.Fatalf("Expect values=%v got %v", eValues, values)
	}
}
