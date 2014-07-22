package main

import (
	"fmt"
	"os"
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
		Summary: "List the current state of units in the cluster",
		Usage:   "[--no-legend] [-l|--full] [--fields]",
		Description: `Lists the state of all units in the cluster loaded onto a machine.

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
			us := j.UnitState
			if us == nil || us.UnitHash == "" {
				return "-"
			}
			if !full {
				return us.UnitHash[:7]
			}
			return us.UnitHash
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

	jobs, err := cAPI.Jobs()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of units from repository: %v\n", err)
		return 1
	}

	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, j := range jobs {
		var f []string
		for _, c := range cols {
			f = append(f, listUnitsFields[c](&j, sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}

func jobToFieldKeys(m map[string]jobToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	return
}
