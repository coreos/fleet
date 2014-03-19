package unit

// Fleet specific unit file requirement keys.
// "X-" prefix only appears in unit file and dropped
// in code before value is used.
const (
	// Require the unit be scheduled to a specific machine defined by given boot ID.
	FleetXConditionMachineBootID = "ConditionMachineBootID"
	// Limit eligible machines to the one that hosts a specific unit.
	FleetXConditionMachineOf = "ConditionMachineOf"
	// Prevent a unit from being collocated with other units using glob-matching on the other unit names.
	FleetXConflicts = "Conflicts"
)
