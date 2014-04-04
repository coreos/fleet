package pkg

import (
	"testing"
)

// TestParseFilepath tests parsing filepath
func TestParseFilepath(t *testing.T) {
	path := ParseFilepath("~/")
	if path[0] != '/' {
		t.Fatal("fail to parse ~")
	}
}
