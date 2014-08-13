package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/job"
)

var (
	flagLines  int
	flagFollow bool
	cmdJournal = &Command{
		Name:    "journal",
		Summary: "Print the journal of a unit in the cluster to stdout",
		Usage:   "[--lines=N] [-f|--follow] job",
		Run:     runJournal,
		Description: `Outputs the journal of a unit by connecting to the machine that the unit occupies.

Read the last 10 lines:
	fleetctl journal foo.service

Read the last 100 lines:
	fleetctl journal --lines 100 foo.service`,
	}
)

func init() {
	cmdJournal.Flags.IntVar(&flagLines, "lines", 10, "Number of recent log lines to return")
	cmdJournal.Flags.BoolVar(&flagFollow, "follow", false, "Continuously print new entries as they are appended to the journal.")
	cmdJournal.Flags.BoolVar(&flagFollow, "f", false, "Shorthand for --follow")
}

func runJournal(args []string) (exit int) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		return 1
	}
	jobName := unitNameMangle(args[0])

	su, err := cAPI.ScheduledUnit(jobName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving Job %s: %v", jobName, err)
		return 1
	}
	if su == nil {
		fmt.Fprintf(os.Stderr, "Unit %s does not exist.\n", jobName)
		return 1
	} else if su.State == nil || *su.State == job.JobStateInactive {
		fmt.Fprintf(os.Stderr, "Unit %s does not appear to be running.\n", jobName)
		return 1
	}

	command := fmt.Sprintf("journalctl --unit %s --no-pager -n %d", jobName, flagLines)
	if flagFollow {
		command += " -f"
	}

	return runCommand(command, su.TargetMachineID)
}
