package machine

import (
	"io/ioutil"
	"strings"
)

const boot_id_path = "/proc/sys/kernel/random/boot_id"

type Machine struct {
	BootId string
}

func New(bootId string) (m *Machine) {
	return &Machine{bootId}
}

func (m *Machine) String() string {
	return m.BootId
}

func ReadLocalBootId() string {
	id, err := ioutil.ReadFile(boot_id_path)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(id))
}
