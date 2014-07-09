package registry

func (r *EtcdRegistry) LockEngine(context string) *TimedResourceMutex {
	return r.lockResource("engine", "leader", context)
}
