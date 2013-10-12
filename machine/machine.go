package machine

import (
	"io/ioutil"
	"strings"
)

type Machine struct {
	BootId string
}

const boot_id_path = "/proc/sys/kernel/random/boot_id"

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

func (m *Machine) String() string {
	return m.BootId
}
