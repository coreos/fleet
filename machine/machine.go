package machine

import (
	"io/ioutil"
	"net"
	"strings"
)

const boot_id_path = "/proc/sys/kernel/random/boot_id"

type Addr struct {
	Addr    string `json:"addr"`
	Network string `json:"network"`
}

type Machine struct {
	BootId string
}

func (m *Machine) String() string {
	return m.BootId
}

func (m *Machine) GetAddresses() []Addr {
	var addrs []Addr
	ifs, err := net.Interfaces()

	if err != nil {
		panic(err)
	}

	shouldAppend := func(i net.Interface) bool {
		if (i.Flags & net.FlagLoopback) == net.FlagLoopback {
			return false
		}

		if (i.Flags & net.FlagUp) != net.FlagUp {
			return false
		}

		return true
	}

	for _, k := range ifs {
		if shouldAppend(k) != true {
			continue
		}
		kaddrs, _ := k.Addrs()
		for _, j := range kaddrs {
			if strings.HasPrefix(j.String(), "fe80::") == true {
				continue
			}
			addrs = append(addrs, Addr{j.String(), j.Network()})
		}
	}

	return addrs
}

func New(bootId string) (m *Machine) {
	m = &Machine{}

	if len(bootId) != 0 {
		m.BootId = bootId
	}

	id, err := ioutil.ReadFile(boot_id_path)
	if err != nil {
		panic(err)
	}
	m.BootId = strings.TrimSpace(string(id))

	return m
}
