package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/fleet/job"
)

const (
	defaultListUnitFilesFields = "unit,hash,dstate"
)

var (
	listUnitFilesFieldsFlag string
	cmdListUnitFiles        = &Command{
		Name:        "list-unit-files",
		Summary:     "List the units that exist in the cluster.",
		Usage:       "[--fields]",
		Description: `Lists all unit files that exist in the cluster (whether or not they are loaded onto a machine).`,
		Run:         runListUnitFiles,
	}
	listUnitFilesFields = map[string]jobToField{
		"unit": func(j *job.Job, full bool) string {
			return j.Name
		},
		"dstate": func(j *job.Job, full bool) string {
			return string(j.TargetState)
		},
		"hash": func(j *job.Job, full bool) string {
			if !full {
				return j.Unit.Hash().Short()
			}
			return j.Unit.Hash().String()
		},
	}
)

func init() {
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnitFiles.Flags.StringVar(&listUnitFilesFieldsFlag, "fields", defaultListUnitFilesFields, fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(jobToFieldKeys(listUnitFilesFields), ",")))
}

func runListUnitFiles(args []string) (exit int) {
	if listUnitFilesFieldsFlag == "" {
		fmt.Fprintf(os.Stderr, "Must define output format\n")
		return 1
	}

	cols := strings.Split(listUnitFilesFieldsFlag, ",")
	for _, s := range cols {
		if _, ok := listUnitFilesFields[s]; !ok {
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
			f = append(f, listUnitFilesFields[c](&j, sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}
