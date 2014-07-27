package event

var (
	GlobalEvent = Event("GlobalEvent")
	JobEvent    = Event("JobEvent")
)

type Event string
