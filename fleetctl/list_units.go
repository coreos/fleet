package main

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:	"list-units",
		Usage:	"Enumerate units loaded in the cluster",
		Description: `Lists all units submitted or started on the cluster.

For easily parsable output, you can remove the column headers:
fleetctl list-units --no-legend

Output the list without ellipses:
fleetctl list-units --full`,
		Action:	listUnitsAction,
		Flags: []cli.Flag{
			cli.BoolFlag{"full, l", "Do not ellipsize fields on output"},
			cli.BoolFlag{"no-legend", "Do not print a legend (column headers)"},
		},
	}
}

func listUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	if !c.Bool("no-legend") {
		fmt.Fprintln(out, "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")
	}

	names := make(map[string]string, 0)
	sortable := make(sort.StringSlice, 0)

	for _, p := range r.GetAllPayloads() {
		if _, ok := names[p.Name]; !ok {
			names[p.Name] = p.Description()
			sortable = append(sortable, p.Name)
		}
	}

	for _, j := range r.GetAllJobs() {
		if _, ok := names[j.Name]; !ok {
			var description string
			if j.Payload != nil {
				description = j.Payload.Description()
			}
			names[j.Name] = description
			sortable = append(sortable, j.Name)
		}
	}

	sortable.Sort()

	full := c.Bool("full")
	for _, name := range sortable {
		state := r.GetJobState(name)
		description := names[name]
		printJobState(name, description, state, full)
	}

	out.Flush()
}

func printJobState(name, description string, js *job.JobState, full bool) {
	loadState := "-"
	activeState := "-"
	subState := "-"
	mach := "-"

	if js != nil {
		loadState = js.LoadState
		activeState = js.ActiveState
		subState = js.SubState

		if js.MachineState != nil {
			mach = js.MachineState.BootId
			if !full {
				mach = ellipsize(mach, 8)
			}
			if len(js.MachineState.PublicIP) > 0 {
				mach = fmt.Sprintf("%s/%s", mach, js.MachineState.PublicIP)
			}
		}
	}

	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\n", name, loadState, activeState, subState, description, mach)
}
