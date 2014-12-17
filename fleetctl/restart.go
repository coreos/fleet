package main

import (
	"os"
	"time"

	"github.com/coreos/fleet/schema"
)

var (
	flagRolling    bool
	cmdRestartUnit = &Command{
		Name:    "restart",
		Summary: "Instruct systemd to restart one or more units in the cluster.",
		Usage:   "[--rolling] [--block-attempts=N] [--ssh-port=N] UNIT...",
		Description: `Restart one or more units running in the cluster.

Instructs systemd on the host machine to restart the unit, deferring to systemd
completely for any custom start and stop directives (i.e. ExecStop or ExecStart
options in the unit file).

Restart a single unit:
  fleetctl restart foo.service

Restart an entire directory of units with glob matching, one at a time:
  fleetctl restart --rolling myservice/*`,
		Run: runRestartUnit,
	}
)

func init() {
	cmdRestartUnit.Flags.IntVar(&sharedFlags.BlockAttempts, "block-attempts", 0, "Run the restart command, performing up to N attempts before giving up. A value of 0 indicates no limit.")
	cmdRestartUnit.Flags.BoolVar(&flagRolling, "rolling", false, "Restart each unit one at a time.")
	cmdRestartUnit.Flags.IntVar(&sharedFlags.sshPort, "ssh-port", 22, "Use this SSH port to connect to host machine.")
}

func runRestartUnit(args []string) (exit int) {
	units, err := findUnits(args)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	if !flagRolling {
		errchan := waitForUnitsToRestart(units, sharedFlags.BlockAttempts, os.Stdout)
		for err := range errchan {
			stderr("Error waiting for units: %v", err)
			exit = 1
		}
	} else {
		for _, unit := range units {
			if suToGlobal(unit) {
				stderr("Unable to restart global unit %s.", unit.Name)
			} else {
				rollingRestart(unit, sharedFlags.BlockAttempts)
			}
		}
	}
	return
}

func rollingRestart(unit schema.Unit, maxAttempts int) (ret bool) {
	sleep := 500 * time.Millisecond
	if maxAttempts < 1 {
		for {
			if assertUnitRestart(unit, out) {
				ret = true
				return
			}
			time.Sleep(sleep)
		}
		stderr("Error restarting unit %s", unit.Name)
	} else {
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if assertUnitRestart(unit, out) {
				ret = true
				return
			}
			time.Sleep(sleep)
		}
	}
	return
}
