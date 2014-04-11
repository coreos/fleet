package main

import (
	"fmt"
	"os"
	"path"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newStatusUnitsCommand() cli.Command {
	return cli.Command{
		Name:  "status",
		Usage: "Output the status of one or more units in the cluster",
		Description: `Output the status of one or more units currently running in the cluster.
Supports glob matching of units in the current working directory or matches
previously started units.

Show status of a single unit:
fleetctl status foo.service

Show status of an entire directory with glob matching:
fleetctl status myservice/*`,
		Action: statusUnitsAction,
	}
}

func statusUnitsAction(c *cli.Context) {
	for i, v := range c.Args() {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := path.Base(v)
		printUnitStatus(c, name)
	}
}

func printUnitStatus(c *cli.Context, jobName string) {
	j := registryCtl.GetJob(jobName)
	if j == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not exist.\n", jobName)
		os.Exit(1)
	} else if j.PayloadState == nil {
		fmt.Fprintf(os.Stderr, "Job %s does not appear to be running.\n", jobName)
		os.Exit(1)
	}

	cmd := fmt.Sprintf("systemctl status -l %s", jobName)
	retcode, err := runCommand(cmd, j.PayloadState.MachineState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed running command over SSH: %v\n", err)
		os.Exit(1)
	}

	os.Exit(retcode)
}
