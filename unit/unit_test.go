package unit

import (
	"testing"
)

const (
	// $ echo -n "foo" | sha1sum
	// 0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33 -
	testData      = "foo"
	testShaString = "0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"
)

func TestUnitHash(t *testing.T) {
	u := NewUnit(testData)
	h := u.Hash()
	if h.String() != testShaString {
		t.Fatalf("Unit Hash (%s) does not match expected (%s)", h.String(), testShaString)
	}

	eh := &Hash{}
	if !eh.Empty() {
		t.Fatalf("Empty hash check failed: %v", eh.Empty())
	}
}
