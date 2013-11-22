package unit

type SystemdService struct {
	manager *SystemdManager
	name string
}

func NewSystemdService(manager *SystemdManager, name string) *SystemdService {
	return &SystemdService{manager, name}
}

func (ss *SystemdService) Name() string {
	return ss.name
}

func (ss *SystemdService) State() (string, []string, error) {
	state, err := ss.manager.getUnitState(ss.name)
	if err != nil {
		return "", nil, err
	}

	return state, make([]string, 0), nil
}

func (ss *SystemdService) Payload() (string, error) {
	return ss.manager.readUnit(ss.Name())
}
