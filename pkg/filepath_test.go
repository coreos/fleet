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

	// parse filepaths of corner case
	path = ParseFilepath("~")
	if path[0] != '/' {
		t.Fatal(path)
		t.Fatal("fail to parse ~")
	}
	path = ParseFilepath("~~")
	if path != "~~" {
		t.Fatal("fail to parse ~~ correctly")
	}
}
