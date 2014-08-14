package main

import (
	"fmt"
	"os"
)

var cmdVerifyUnit = &Command{
	Name:    "verify",
	Summary: "DEPRECATED - No longer works",
	Usage:   "UNIT",
	Run:     runVerifyUnit,
}

func runVerifyUnit(args []string) (exit int) {
	fmt.Fprintln(os.Stderr, "WARNING: The signed/verified units feature is DEPRECATED and cannot be used.")
	return 2
}
