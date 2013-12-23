package machine

import (
	"testing"
)

func TestNew(t *testing.T) {
	m1 := New("XXX", "1.2.3.4", map[string]string{"foo": "bar"})

	if m1.BootId != "XXX" {
		t.Fatal("machine.Machine.BootId != 'XXX'")
	}

	if m1.PublicIP != "1.2.3.4" {
		t.Fatal("machine.Machine.PublicIP != '1.2.3.4'")
	}

	if len(m1.Metadata) != 1 || m1.Metadata["foo"] != "bar" {
		t.Fatal("machine.Machine.Metadata != '{foo: bar}'")
	}
}

func TestStringEncoding(t *testing.T) {
	m1 := Machine{"XXX", "1.2.3.4", make(map[string]string, 0)}
	result := m1.String()
	expect := "XXX"
	if result != expect {
		t.Fatalf("machine.Machine.String() returned '%s', expected '%s'", result, expect)
	}
}
