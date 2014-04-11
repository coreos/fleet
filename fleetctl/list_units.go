package main

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:  "list-units",
		Usage: "Enumerate units loaded in the cluster",
		Description: `Lists all units submitted or started on the cluster.

For easily parsable output, you can remove the column headers:
fleetctl list-units --no-legend

Output the list without ellipses:
fleetctl list-units --full`,
		Action: listUnitsAction,
		Flags: []cli.Flag{
			cli.BoolFlag{"full, l", "Do not ellipsize fields on output"},
			cli.BoolFlag{"no-legend", "Do not print a legend (column headers)"},
		},
	}
}

func listUnitsAction(c *cli.Context) {
	if !c.Bool("no-legend") {
		fmt.Fprintln(out, "UNIT\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")
	}

	names, sortable := findAllUnits()

	full := c.Bool("full")
	for _, name := range sortable {
		var ps *job.PayloadState
		j := registryCtl.GetJob(name)
		if j != nil {
			ps = j.PayloadState
		}
		description := names[name]
		printPayloadState(name, description, ps, full)
	}

	out.Flush()
}

func findAllUnits() (names map[string]string, sortable sort.StringSlice) {
	names = make(map[string]string, 0)
	sortable = make(sort.StringSlice, 0)

	for _, j := range registryCtl.GetAllJobs() {
		if _, ok := names[j.Name]; !ok {
			var description string
			description = j.Payload.Unit.Description()
			names[j.Name] = description
			sortable = append(sortable, j.Name)
		}
	}

	sortable.Sort()

	return names, sortable
}

func printPayloadState(name, description string, js *job.PayloadState, full bool) {
	loadState := "-"
	activeState := "-"
	subState := "-"
	mach := "-"

	if description == "" {
		description = "-"
	}

	if js != nil {
		loadState = js.LoadState
		activeState = js.ActiveState
		subState = js.SubState

		if js.MachineState != nil {
			mach = machineFullLegend(*js.MachineState, full)
		}
	}

	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\n", name, loadState, activeState, subState, description, mach)
}
