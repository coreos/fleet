package machine

import (
	"testing"
)

const freeMOutput = `
             total       used       free     shared    buffers     cached
Mem:           998         93        904          0          4         52
-/+ buffers/cache:         37        961
Swap:            0          0          0
`

func TestParseFreeMOutput(t *testing.T) {
	mem, err := parseFreeMOutput([]byte(freeMOutput))
	if err != nil {
		t.Fatalf("Couldn't parse command output: %v", err)
	}

	if mem != 998 {
		t.Errorf("expected 998 MB, got %d", mem)
	}
}
