package main

import (
	"fmt"
	"sort"

	"github.com/coreos/fleet/job"
)

var cmdListUnits = &Command{
	Name:    "list-units",
	Summary: "Enumerate units loaded in the cluster",
	Usage:   "[--no-legend] [--full]",
	Description: `Lists all units submitted or started on the cluster.

For easily parsable output, you can remove the column headers:
	fleetctl list-units --no-legend

Output the list without ellipses:
	fleetctl list-units --full`,
	Run: runListUnits,
}

func init() {
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnits.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
}

func runListUnits(args []string) (exit int) {
	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, "UNIT\tSTATE\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")
	}

	jobs, sortable := findAllUnits()

	for _, name := range sortable {
		j := jobs[name]
		printPayloadState(name, j.Payload.Unit.Description(), j.State, j.PayloadState, sharedFlags.Full)
	}

	out.Flush()
	return
}

func findAllUnits() (jobs map[string]job.Job, sortable sort.StringSlice) {
	jobs = make(map[string]job.Job, 0)
	sortable = make(sort.StringSlice, 0)

	for _, j := range registryCtl.GetAllJobs() {
		jobs[j.Name] = j
		sortable = append(sortable, j.Name)
	}

	sortable.Sort()

	return jobs, sortable
}

func printPayloadState(name, description string, js *job.JobState, ps *job.PayloadState, full bool) {
	jobState := "-"
	loadState := "-"
	activeState := "-"
	subState := "-"
	mach := "-"

	if description == "" {
		description = "-"
	}

	if js != nil {
		jobState = string(*js)
	}

	if ps != nil {
		loadState = ps.LoadState
		activeState = ps.ActiveState
		subState = ps.SubState

		if ps.MachineState != nil {
			mach = machineFullLegend(*ps.MachineState, full)
		}
	}

	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", name, jobState, loadState, activeState, subState, description, mach)
}
