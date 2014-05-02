package job

import (
	"testing"
)

func TestNewPathType(t *testing.T) {
	jt := newJobType(PathUnit)

	if j, ok := jt.(*PathType); !ok {
		t.Errorf("Unexpected job type %q", j)
	}
}

func TestNewServiceType(t *testing.T) {
	jt := newJobType(ServiceUnit)

	if j, ok := jt.(*ServiceType); !ok {
		t.Errorf("Unexpected job type %q", j)
	}
}

func TestNewJobUnitType(t *testing.T) {
	types := []string{SocketUnit, TimerUnit}

	for _, tt := range types {
		jt := newJobType(tt)

		if j, ok := jt.(*JobUnitType); !ok {
			t.Errorf("Unexpected job type %q for %s", j, tt)
		}
	}
}
