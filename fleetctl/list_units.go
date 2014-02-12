package main

import (
	"fmt"
	"sort"

	"github.com/codegangsta/cli"

	"github.com/coreos/fleet/job"
)

func newListUnitsCommand() cli.Command {
	return cli.Command{
		Name:   "list-units",
		Usage:  "Enumerate units loaded in the cluster",
		Action: listUnitsAction,
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

	names := make(map[string]bool, 0)
	sortable := make(sort.StringSlice, 0)

	for _, p := range r.GetAllPayloads() {
		if _, ok := names[p.Name]; !ok {
			names[p.Name] = true
			sortable = append(sortable, p.Name)
		}
	}

	for _, j := range r.GetAllJobs() {
		if _, ok := names[j.Name]; !ok {
			names[j.Name] = true
			sortable = append(sortable, j.Name)
		}
	}

	sortable.Sort()

	full := c.Bool("full")
	for _, name := range sortable {
		state := r.GetJobState(name)
		printJobState(name, state, full)
	}

	out.Flush()
}

func printJobState(name string, js *job.JobState, full bool) {
	loadState := "-"
	activeState := "-"
	subState := "-"
	mach := "-"

	if js != nil {
		loadState = js.LoadState
		activeState = js.ActiveState
		subState = js.SubState

		if js.Machine != nil {
			mach = js.Machine.String()
			if !full {
				mach = ellipsize(mach, 8)
			}
			if len(js.Machine.PublicIP) > 0 {
				mach = fmt.Sprintf("%s/%s", mach, js.Machine.PublicIP)
			}
		}
	}

	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t-\t%s\n", name, loadState, activeState, subState, mach)
}
