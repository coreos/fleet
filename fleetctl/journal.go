package main

import (
	"fmt"
	"os"
)

var (
	flagLines  int
	flagFollow bool
	cmdJournal = &Command{
		Name:    "journal",
		Summary: "Print the journal of a unit in the cluster to stdout",
		Usage:   "[--lines=N] [--follow] job",
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
}

func runJournal(args []string) (exit int) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		return 1
	}
	jobName := args[0]

	j := registryCtl.GetJob(jobName)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not exist.\n", jobName)
		os.Exit(1)
	} else if j.PayloadState == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not appear to be running.\n", jobName)
		return 1
	}

	command := fmt.Sprintf("journalctl -u %s --no-pager -l -n %d", jobName, flagLines)
	if flagFollow {
		command += " -f"
	}

	retcode, err := runCommand(command, j.PayloadState.MachineState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed running command over SSH: %v\n", err)
		return 1
	}

	return retcode
}
