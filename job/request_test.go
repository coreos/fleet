package job

import (
	"testing"
)

func TestNewJobRequestNil(t *testing.T) {
	jr := NewJobRequest(nil, nil)

	if jr.Machines != nil {
		t.Error("Machines is not nil")
	}

	if jr.Payloads != nil {
		t.Error("Payloads is not nil")
	}
}
