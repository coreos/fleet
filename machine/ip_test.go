package machine

import (
	"net"
	"testing"
)

func TestIPAddress(t *testing.T) {
	a := IPAddress{"127.0.0.1/32", "ip+net"}

	if a.String() != "127.0.0.1/32" {
		t.Fatal("addr.String() != '127.0.0.1/32'")
	}

	if a.Network() != "ip+net" {
		t.Fatal("addr.Network() != 'ip+net'")
	}
}

type FakeInterface struct {
	addrs []net.Addr
}

func (f *FakeInterface) Addrs() ([]net.Addr, error) {
	return f.addrs, nil
}

func TestGetAddrsFromInterfaceEmpty(t *testing.T) {
	addrs := []net.Addr{}
	iface := FakeInterface{addrs}
	result := getAddrsFromInterface(&iface)

	if len(result) != 0 {
		t.Error("List of Addr objects is not of length 0")
	}
}

func TestGetAddrsFromInterfaceSingle(t *testing.T) {
	addrs := []net.Addr{&IPAddress{"127.0.0.1/32", "ip+net"}}
	iface := FakeInterface{addrs}
	result := getAddrsFromInterface(&iface)

	if len(result) != 1 {
		t.Error("List of Addr objects is not of length 1")
	}

	if result[0].String() != "127.0.0.1/32" {
		t.Errorf("Addr.String() returned %s, expected '127.0.0.1/32'", result[0].String())
	}
}

func TestGetAddrsFromInterfaceFilterFE80(t *testing.T) {
	addrs := []net.Addr{&IPAddress{"fe80::12", "ip+net"}}
	iface := FakeInterface{addrs}
	result := getAddrsFromInterface(&iface)

	if len(result) != 0 {
		t.Error("List of Addr objects is not of length 0")
	}
}

func TestFilterInterfaces(t *testing.T) {
	hwaddr, _ := net.ParseMAC("01:23:45:67:89:ab")
	ifaces := []net.Interface{
		net.Interface{0, 0, "eth0", hwaddr, 0},
		net.Interface{1, 0, "eth1", hwaddr, net.FlagUp | net.FlagLoopback},
		net.Interface{2, 0, "eth2", hwaddr, net.FlagUp},
	}

	output := filterInterfaces(ifaces)

	if len(output) != 1 || output[0].Name != "eth2"{
		t.Error("filterInterfaces failed to return proper net.Interface objects")
	}
}
