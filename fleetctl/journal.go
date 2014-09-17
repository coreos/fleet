package main

import (
	"fmt"

	"github.com/coreos/fleet/pkg"
)

var (
	flagLines  int
	flagFollow bool
	cmdJournal = &Command{
		Name:    "journal",
		Summary: "Print the journal of a unit in the cluster to stdout",
		Usage:   "[--lines=N] [-f|--follow] <unit>",
		Run:     runJournal,
		Description: `Outputs the journal of a unit by connecting to the machine that the unit occupies.

Read the last 10 lines:
	fleetctl journal foo.service

Read the last 100 lines:
	fleetctl journal --lines 100 foo.service`,
	}
)

func init() {
	cmdJournal.Flags.StringVar(&sharedFlags.Machine, "machine", "", "Fetch the logs for a unit from a specific machine.")
	cmdJournal.Flags.IntVar(&flagLines, "lines", 10, "Number of recent log lines to return")
	cmdJournal.Flags.BoolVar(&flagFollow, "follow", false, "Continuously print new entries as they are appended to the journal.")
	cmdJournal.Flags.BoolVar(&flagFollow, "f", false, "Shorthand for --follow")
}

func runJournal(args []string) (exit int) {
	if len(args) != 1 {
		stderr("One unit file must be provided.")
		return 1
	}
	name := unitNameMangle(args[0])

	states, err := cAPI.UnitStates()
	if err != nil {
		stderr("Error retrieving unit %s: %v", name, err)
		return 1
	}

	machines := pkg.NewUnsafeSet()
	for _, s := range states {
		if s.Name != name {
			continue
		}

		machines.Add(s.MachineID)
	}

	mLen := machines.Length()
	if mLen == 0 {
		stderr("No state found for unit.")
		return 1
	}

	var target string

	if sharedFlags.Machine != "" {
		if !machines.Contains(sharedFlags.Machine) {
			stderr("Unable to find state for unit on provided machine.")
			return 1
		}
		target = sharedFlags.Machine

	} else {
		if mLen > 1 {
			stderr("Multiple machines reporting state for unit. Specify machine using the --machine flag.")
			return 1
		}
		target = machines.Values()[0]
	}

	command := fmt.Sprintf("journalctl --unit %s --no-pager -n %d", name, flagLines)
	if flagFollow {
		command += " -f"
	}

	return runCommand(command, target)
}
