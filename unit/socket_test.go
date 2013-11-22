package unit

import (
	"testing"
)

func TestNewSystemdSockete(t *testing.T) {
	contents := `[Unit]
Description=Example Socket File

[Socket]
ListenStream=1.2.3.4:23
ListenDatagram=1.2.3.4:24
#ListenDatagram=1.2.3.4:25
ListenSequentialPacket=/var/run/mysqld.sock
`
	sf := NewSystemdSocket(nil, "foo.socket", contents)

	expected := []ListenSocket{
		ListenSocket{"tcp", "1.2.3.4:23"},
		ListenSocket{"udp", "1.2.3.4:24"},
		ListenSocket{"unix", "/var/run/mysqld.sock"},
	}

	if len(sf.Sockets) != len(expected) {
		t.Fatalf("Expected %d sockets, received %d", len(expected), len(sf.Sockets))
	}

	if sf.Name() != "foo.socket" {
		t.Errorf("SystemdSocket has incorrect name")
	}

	for i, expect := range expected {
		if sf.Sockets[i].Type != expect.Type {
			t.Errorf("Socket type '%s' does not match expected '%s'", sf.Sockets[i].Type, expect.Type)
		}

		if sf.Sockets[i].Address != expect.Address {
			t.Errorf("Socket type '%s' does not match expected '%s'", sf.Sockets[i].Address, expect.Address)
		}
	}
}

func TestNewListenSocketFromListenConfig(t *testing.T) {
	goodLines := []string{
		"ListenStream=1.2.3.4:23",
		"ListenDatagram=24",
		"ListenSequentialPacket=/var/run/mysqld.sock",
	}

	for _, line := range goodLines {
		_, err := NewListenSocketFromListenConfig(line)
		if err != nil {
			t.Errorf("Parsing of good line failed: %s", line)
		}
	}

	badLines := []string{
		"ListenSocket=1.2.3.4:23", //ListenSocket is an invalid key
		"FooBar=1.2.3.4:23", //FooBar is an invalid key
	}

	for _, line := range badLines {
		_, err := NewListenSocketFromListenConfig(line)
		if err == nil {
			t.Errorf("Parsing of bad line succeeded: %s", line)
		}
	}

}
