package registry

type Interface struct {
	Name string `json:"name"`
	MTU int `json:"mtu"`
	HardwareAddr string `json:"hardware_addr"`
	Addrs []*Addr `json:"addrs"`
}

type Addr struct {
	Addr string `json:"addr"`
	Network string `json:"network"`
}

type Machine struct {
	Interfaces []*Interface `json:"interfaces"`
}
