package agent

import (
	"testing"
)

// Assert that an existing conflict is triggered against the potential job name
func TestHasConflictExistingMatch(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"b"})

	matched, name := state.HasConflict("b", []string{})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}
}

// Assert that a potential conflict is triggered against the existing job name
func TestHasConflictPotentialMatch(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{})

	matched, name := state.HasConflict("b", []string{"a"})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}
}

// Assert that a existing jobs and potential jobs that do not conflict do not
// trigger a match
func TestHasConflictNoMatch(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"b"})

	matched, _ := state.HasConflict("c", []string{"d"})
	if matched {
		t.Errorf("Expected no match")
	}
}

// Assert that our glob-parser can handle relatively-complex matching
func TestHasConflictComplexGlob(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"*.[1-9].service"})

	matched, name := state.HasConflict("web.2.service", []string{})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}

	matched, _ = state.HasConflict("app.99.service", []string{})
	if matched {
		t.Errorf("Expected no conflict")
	}
}

// Assert that a conflict is truly gone when DropJobConflicts is called
func TestHasConflictDropped(t *testing.T) {
	state := NewState()
	state.TrackJobConflicts("a", []string{"b"})

	matched, name := state.HasConflict("b", []string{})
	if !matched || name != "a" {
		t.Errorf("Expected conflict with 'a'")
	}

	state.DropJobConflicts("a")
	matched, _ = state.HasConflict("b", []string{})
	if matched {
		t.Errorf("Expected no conflict")
	}
}
