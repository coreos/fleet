package machine

import (
	"log"
	"net"
	"strings"
)

type IPAddress struct {
	address string `json:"address"`
	network string `json:"network"`
}

// This exists to implement the net.Addr interface
func (ip *IPAddress) String() string {
	return ip.address
}

// This exists to implement the net.Addr interface
func (ip *IPAddress) Network() string {
	return ip.network
}

// This exists so we can mock out the response from net.Interfaces
// in our testing
type Interface interface {
	Addrs() ([]net.Addr, error)
}

func (m *Machine) GetAddresses() []IPAddress {
	ifaces := getLocalInterfaces()
	ifaces = filterInterfaces(ifaces)

	var addrs []IPAddress
	for _, iface := range ifaces {
		for _, addr := range getAddrsFromInterface(Interface(&iface)) {
			addrs = append(addrs, IPAddress{addr.String(), addr.Network()})
		}
	}

	return addrs
}

func filterInterfaces(ifaces []net.Interface) []net.Interface {
	// Filter out the loopback device and any down interfaces
	filter := func(i net.Interface) bool {
		if (i.Flags & net.FlagLoopback) == net.FlagLoopback {
			return false
		} else if (i.Flags & net.FlagUp) != net.FlagUp {
			return false
		} else {
			return true
		}
	}

	ret := make([]net.Interface, 0)
	for _, iface := range ifaces {
		if filter(iface) {
			ret = append(ret, iface)
		}
	}

	return ret
}

func getAddrsFromInterface(iface Interface) []net.Addr {
	// Filter out the link-local IPv6 addresses
	filter := func(a net.Addr) bool {
		if strings.HasPrefix(a.String(), "fe80::") == true {
			return false
		} else {
			return true
		}
	}

	var ret []net.Addr
	addrs, _ := iface.Addrs()
	for _, v := range addrs {
		if filter(v) {
			ret = append(ret, v)
		}
	}

	return ret
}

func getLocalInterfaces() []net.Interface {
	ifaces, err := net.Interfaces()

	if err != nil {
		log.Fatal("Failed to get local interfaces")
		return make([]net.Interface, 0)
	}

	return ifaces
}
