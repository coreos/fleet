package machine

import (
	"testing"
)

func TestNew(t *testing.T) {
	m1 := New("XXX", "1.2.3.4")
	m2 := Machine{"XXX", "1.2.3.4"}

	if *m1 != m2 {
		t.Error("machine.New factory failed to produce appropriate machine.Machine")
	}

	if m1.BootId != "XXX" {
		t.Fatal("machine.Machine.BootId != 'XXX'")
	}

	if m1.PublicIP != "1.2.3.4" {
		t.Fatal("machine.Machine.PublicIP != '1.2.3.4'")
	}
}

func TestStringEncoding(t *testing.T) {
	m1 := Machine{"XXX", "1.2.3.4"}
	result := m1.String()
	expect := "XXX"
	if result != expect {
		t.Fatalf("machine.Machine.String() returned '%s', expected '%s'", result, expect)
	}
}
