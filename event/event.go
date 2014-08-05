package event

var (
	// Occurs when any Job's target is touched
	JobTargetChangeEvent = Event("JobTargetChangeEvent")
	// Occurs when any Job's target state is touched
	JobTargetStateChangeEvent = Event("JobTargetStateChangeEvent")
)

type Event string
