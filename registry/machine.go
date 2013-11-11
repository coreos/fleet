package registry

import (
	"io/ioutil"
	"strings"
)

const boot_id_path = "/proc/sys/kernel/random/boot_id"

type Addr struct {
	Addr string `json:"addr"`
	Network string `json:"network"`
}

type Machine struct {
	BootId string
}

func (m *Machine) String() string {
	return m.BootId
}

func NewMachine(bootId string) (m *Machine) {
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
