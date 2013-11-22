package unit

import (
	"testing"
)

func TestSerializeDbusPath(t *testing.T) {
	input := "/silly-path/to@a/unit..service"
	output := serializeDbusPath(input)
	expected := "/silly_2dpath/to_40a/unit_2e_2eservice"

	if output != expected {
		t.Fatalf("Output '%s' did not match expected '%s'", output, expected)
	}
}
