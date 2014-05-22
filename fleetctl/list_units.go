package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/unit"
)

var cmdListUnits = &Command{
	Name:    "list-units",
	Summary: "Enumerate units loaded in the cluster",
	Usage:   "[--no-legend] [-l|--full]",
	Description: `Lists all units submitted or started on the cluster.

For easily parsable output, you can remove the column headers:
	fleetctl list-units --no-legend

Output the list without ellipses:
	fleetctl list-units --full`,
	Run: runListUnits,
}

func init() {
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListUnits.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
}

func runListUnits(args []string) (exit int) {
	jobs, sortable, err := findAllUnits()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of units from repository: %v\n", err)
		return 1
	}

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, "UNIT\tSTATE\tLOAD\tACTIVE\tSUB\tDESC\tMACHINE")
	}

	for _, name := range sortable {
		j := jobs[name]
		printUnitState(name, j.Unit.Description(), j.State, j.UnitState, sharedFlags.Full)
	}

	out.Flush()
	return
}

// findAllUnits returns a map describing all the Jobs in the Registry, and a
// sort.StringSlice containing their names in sorted order.
// It returns any error encountered in communicating with the Registry.
func findAllUnits() (jobs map[string]job.Job, sortable sort.StringSlice, err error) {
	jobs = make(map[string]job.Job, 0)
	jj, err := registryCtl.GetAllJobs()
	if err != nil {
		return
	}

	for _, j := range jj {
		jobs[j.Name] = j
		sortable = append(sortable, j.Name)
	}

	sortable.Sort()

	return
}

func printUnitState(name, description string, js *job.JobState, us *unit.UnitState, full bool) {
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

	if us != nil {
		loadState = us.LoadState
		activeState = us.ActiveState
		subState = us.SubState

		if us.MachineState != nil {
			mach = machineFullLegend(*us.MachineState, full)
		}
	}

	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", name, jobState, loadState, activeState, subState, description, mach)
}
