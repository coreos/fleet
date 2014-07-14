package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

const (
	defaultListUnitsFields = "unit,dstate,tmachine,state,machine,active"
)

var (
	listUnitsFieldsFlag string
	cmdListUnits        = &Command{
		Name:    "list-units",
		Summary: "Enumerate units loaded in the cluster",
		Usage:   "[--no-legend] [-l|--full]",
		Description: `Lists all units submitted or started on the cluster.

For easily parsable output, you can remove the column headers:
	fleetctl list-units --no-legend

Output the list without ellipses:
	fleetctl list-units --full

Or, choose the columns to display:
	fleetctl list-units --fields=unit,machine`,
		Run: runListUnits,
	}

	listUnitsFields = map[string]jobToField{
		"unit": func(j *job.Job, full bool) string {
			return j.Name
		},
		"state": func(j *job.Job, full bool) string {
			js := j.State
			if js != nil {
				return string(*js)
			}
			return "-"
		},
		"dstate": func(j *job.Job, full bool) string {
			return string(j.TargetState)
		},
		"load": func(j *job.Job, full bool) string {
			us := j.UnitState
			if us == nil {
				return "-"
			}
			return us.LoadState
		},
		"active": func(j *job.Job, full bool) string {
			us := j.UnitState
			if us == nil {
				return "-"
			}
			return us.ActiveState
		},
		"sub": func(j *job.Job, full bool) string {
			us := j.UnitState
			if us == nil {
				return "-"
			}
			return us.SubState
		},
		"desc": func(j *job.Job, full bool) string {
			d := j.Unit.Description()
			if d == "" {
				return "-"
			}
			return d
		},
		"machine": func(j *job.Job, full bool) string {
			us := j.UnitState
			if us == nil || us.MachineID == "" {
				return "-"
			}
			ms := cachedMachineState(us.MachineID)
			if ms == nil {
				ms = &machine.MachineState{ID: us.MachineID}
			}
			return machineFullLegend(*ms, full)
		},
		"tmachine": func(j *job.Job, full bool) string {
			if j.TargetMachineID == "" {
				return "-"
			}
			ms := cachedMachineState(j.TargetMachineID)
			if ms == nil {
				ms = &machine.MachineState{ID: j.TargetMachineID}
			}
			return machineFullLegend(*ms, full)
		},
		"hash": func(j *job.Job, full bool) string {
			if !full {
				return j.Unit.Hash().Short()
			}
			return j.Unit.Hash().String()
		},
	}
)

type jobToField func(j *job.Job, full bool) string

func init() {
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnits.Flags.BoolVar(&sharedFlags.Full, "l", false, "Shorthand for --full")
	cmdListUnits.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnits.Flags.StringVar(&listUnitsFieldsFlag, "fields", defaultListUnitsFields, fmt.Sprintf("Columns to print for each Unit. Valid fields are %q", strings.Join(jobToFieldKeys(listUnitsFields), ",")))
}

func runListUnits(args []string) (exit int) {

	if listUnitsFieldsFlag == "" {
		fmt.Fprintf(os.Stderr, "Must define output format\n")
		return 1
	}

	cols := strings.Split(listUnitsFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listUnitsFields[s]; !ok {
			fmt.Fprintf(os.Stderr, "Invalid key in output format: %q\n", s)
			return 1
		}
	}

	jobs, sortable, err := findAllUnits()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of units from repository: %v\n", err)
		return 1
	}

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, name := range sortable {
		var f []string
		j := jobs[name]
		for _, c := range cols {
			f = append(f, listUnitsFields[c](&j, sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}

// findAllUnits returns a map describing all the Jobs in the Registry, and a
// sort.StringSlice containing their names in sorted order.
// It returns any error encountered in communicating with the Registry.
func findAllUnits() (jobs map[string]job.Job, sortable sort.StringSlice, err error) {
	jobs = make(map[string]job.Job, 0)
	jj, err := cAPI.Jobs()
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

func jobToFieldKeys(m map[string]jobToField) (keys []string) {
	for k, _ := range m {
		keys = append(keys, k)
	}
	return
}
