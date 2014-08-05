package event

var (
	// Occurs on any action in the Job namespace
	JobEvent = Event("JobEvent")
	// Occurs when any Job's target state is set
	JobTargetStateSetEvent = Event("JobTargetStateSetEvent")
)

type Event string
