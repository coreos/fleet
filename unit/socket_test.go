package unit

import (
	"testing"
)

func TestParseSocketFile(t *testing.T) {
	contents := `[Unit]
Description=Example Socket File

[Socket]
ListenStream=23
ListenDatagram=24
#ListenDatagram=1.2.3.4:25
ListenSequentialPacket=/var/run/mysqld.sock
`
	sockets := parseSocketFile(contents)

	expected := []ListenSocket{
		ListenSocket{"tcp", 23},
		ListenSocket{"udp", 24},
	}

	if len(sockets) != len(expected) {
		t.Fatalf("Expected %d sockets, received %d", len(expected), len(sockets))
	}

	for i, expect := range expected {
		if sockets[i].Type != expect.Type {
			t.Errorf("Socket type '%s' does not match expected '%s'", sockets[i].Type, expect.Type)
		}

		if sockets[i].Port != expect.Port {
			t.Errorf("Socket type '%s' does not match expected '%s'", sockets[i].Port, expect.Port)
		}
	}
}

func TestNewListenSocketFromListenConfig(t *testing.T) {
	goodLines := []string{
		"ListenStream=23",
		"ListenDatagram=24",
	}

	for _, line := range goodLines {
		_, err := NewListenSocketFromListenConfig(line)
		if err != nil {
			t.Errorf("Parsing of good line failed: %s", line)
		}
	}

	badLines := []string{
		"ListenSocket=23", //ListenSocket is an invalid key
		"FooBar=23", //FooBar is an invalid key
		"ListenStream=1.2.3.4:23", //coreinit only supports ports
		"ListenSequentialPacket=/var/run/mysqld.sock", //coreinit only support ports
	}

	for _, line := range badLines {
		_, err := NewListenSocketFromListenConfig(line)
		if err == nil {
			t.Errorf("Parsing of bad line succeeded: %s", line)
		}
	}

}
