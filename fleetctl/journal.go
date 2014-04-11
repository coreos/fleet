package main

import (
	"fmt"
	"os"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newJournalCommand() cli.Command {
	return cli.Command{
		Name:   "journal",
		Usage:  "Print the journal of a unit in the cluster to stdout",
		Action: journalAction,
		Description: `Outputs the journal of a unit by connecting to the machine that the unit
occupies.

Read the last 10 lines:
fleetctl journal foo.service

Read the last 100 lines:
fleetctl journal --lines 100 foo.service`,
		Flags: []cli.Flag{
			cli.IntFlag{"lines, n", 10, "Number of recent log lines to return."},
			cli.BoolFlag{"follow, f", "Continuously print new entries as they are appended to the journal."},
		},
	}
}

func journalAction(c *cli.Context) {
	if len(c.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "One unit file must be provided.")
		os.Exit(1)
	}
	jobName := c.Args()[0]

	j := registryCtl.GetJob(jobName)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not exist.\n", jobName)
		os.Exit(1)
	} else if j.PayloadState == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not appear to be running.\n", jobName)
		os.Exit(1)
	}

	cmd := fmt.Sprintf("journalctl -u %s --no-pager -l -n %d", jobName, c.Int("lines"))
	if c.Bool("follow") {
		cmd += " -f"
	}

	retcode, err := runCommand(cmd, j.PayloadState.MachineState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed running command over SSH: %v\n", err)
		os.Exit(1)
	}

	os.Exit(retcode)
}
