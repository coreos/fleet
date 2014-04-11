package event

type EventListener struct {
	Context string
	Handler interface{}
}

func (self *EventListener) String() string {
	if self.Context != "" {
		return self.Context
	} else {
		return "N/A"
	}
}
