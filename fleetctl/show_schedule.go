package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
)

const (
	defaultShowScheduleFields = "unit,dstate,state,tmachine"
)

var (
	showScheduleFieldsFlag string
	cmdShowSchedule        = &Command{
		Name:        "show-schedule",
		Summary:     "Display the scheduling of units that exist in the cluster.",
		Usage:       "[--fields]",
		Description: `Display the current scheduling of units in the cluster.`,
		Run:         runShowSchedule,
	}
	showScheduleFields = map[string]schedUnitToField{
		"unit": func(j job.ScheduledUnit, full bool) string {
			return j.Name
		},
		"dstate": func(j job.ScheduledUnit, full bool) string {
			return string(j.TargetState)
		},
		"state": func(j job.ScheduledUnit, full bool) string {
			js := j.State
			if js != nil {
				return string(*js)
			}
			return "-"
		},
		"tmachine": func(j job.ScheduledUnit, full bool) string {
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

type schedUnitToField func(j job.ScheduledUnit, full bool) string

func init() {
	cmdShowSchedule.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdShowSchedule.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdShowSchedule.Flags.StringVar(&showScheduleFieldsFlag, "fields", defaultShowScheduleFields, fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(schedUnitToFieldKeys(showScheduleFields), ",")))
}

func runShowSchedule(args []string) (exit int) {
	if showScheduleFieldsFlag == "" {
		fmt.Fprintf(os.Stderr, "Must define output format\n")
		return 1
	}

	cols := strings.Split(showScheduleFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := showScheduleFields[s]; !ok {
			fmt.Fprintf(os.Stderr, "Invalid key in output format: %q\n", s)
			return 1
		}
	}

	units, err := cAPI.Schedule()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving list of units from repository: %v\n", err)
		return 1
	}
	if !sharedFlags.NoLegend {
		fmt.Fprintln(out, strings.ToUpper(strings.Join(cols, "\t")))
	}

	for _, u := range units {
		var f []string
		for _, c := range cols {
			f = append(f, showScheduleFields[c](u, sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}

func schedUnitToFieldKeys(m map[string]schedUnitToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	return
}
