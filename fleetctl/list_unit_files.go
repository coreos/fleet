package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/coreos/fleet/job"
)

const (
	defaultListUnitFilesFields = "unit,hash,desc"
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
	listUnitFilesFields = map[string]jobUnitToField{
		"unit": func(j job.Unit, full bool) string {
			return j.Name
		},
		"hash": func(j job.Unit, full bool) string {
			if !full {
				return j.Unit.Hash().Short()
			}
			return j.Unit.Hash().String()
		},
		"desc": func(j job.Unit, full bool) string {
			d := j.Unit.Description()
			if d == "" {
				return "-"
			}
			return d
		},
	}
)

type jobUnitToField func(j job.Unit, full bool) string

func init() {
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.Full, "full", false, "Do not ellipsize fields on output")
	cmdListUnitFiles.Flags.BoolVar(&sharedFlags.NoLegend, "no-legend", false, "Do not print a legend (column headers)")
	cmdListUnitFiles.Flags.StringVar(&listUnitFilesFieldsFlag, "fields", defaultListUnitFilesFields, fmt.Sprintf("Columns to print for each Unit file. Valid fields are %q", strings.Join(jobUnitToFieldKeys(listUnitFilesFields), ",")))
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

	jobs, err := cAPI.Units()

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
			f = append(f, listUnitFilesFields[c](j, sharedFlags.Full))
		}
		fmt.Fprintln(out, strings.Join(f, "\t"))
	}

	out.Flush()
	return
}

func jobUnitToFieldKeys(m map[string]jobUnitToField) (keys []string) {
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return
}
