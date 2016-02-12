package rpc

import (
	"encoding/json"
	"net/http"
)

func dbgHandler(w http.ResponseWriter, r *http.Request) {
	if currentReg == nil {
		return
	}
	e := json.NewEncoder(w)

	data := map[string]interface{}{
		"Units":          currentReg.unitsCache,
		"ScheduledUnits": currentReg.scheduledUnits,
		"Heartbeats":     currentReg.unitHeartbeats,
		"States":         currentReg.unitStates,
	}

	e.Encode(data)
}
