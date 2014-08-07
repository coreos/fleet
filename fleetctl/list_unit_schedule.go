package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

const (
	defaultListUnitScheduleFields = "unit,dstate,state,tmachine"
)

var (
	listUnitScheduleFieldsFlag string
	cmdListUnitSchedule        = &Command{
		Name:        "list-unit-schedule",
		Summary:     "List the scheduling of units that exist in the cluster.",
		Usage:       "[--fields]",
		Description: `Display the current scheduling of units in the cluster.`,
		Run:         runListUnitSchedule,
	}
	listUnitScheduleFields = map[string]jobToField{
		"unit": func(j *job.Job, full bool) string {
			return j.Name
		},
		"dstate": func(j *job.Job, full bool) string {
			return string(j.TargetState)
		},
		"state": func(j *job.Job, full bool) string {
			js := j.State
			if js != nil {
				return string(*js)
			}
			return "-"
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
	}
)

func init() {
	cmdListUnitSchedule.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnitSchedule.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnitSchedule.Flags.StringVar(&listUnitScheduleFieldsFlag, "fields", defaultListUnitScheduleFields, fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(jobToFieldKeys(listUnitScheduleFields), ",")))
}

func runListUnitSchedule(args []string) (exit int) {
	if listUnitScheduleFieldsFlag == "" {
		fmt.Fprintf(os.Stderr, "Must define output format\n")
		return 1
	}

	cols := strings.Split(listUnitScheduleFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listUnitScheduleFields[s]; !ok {
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
		j := j
		var f []string
		for _, c := range cols {
			f = append(f, listUnitScheduleFields[c](&j, sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}
