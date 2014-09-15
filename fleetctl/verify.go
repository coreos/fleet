package main

var cmdVerifyUnit = &Command{
	Name:    "verify",
	Summary: "DEPRECATED - No longer works",
	Usage:   "UNIT",
	Run:     runVerifyUnit,
}

func runVerifyUnit(args []string) (exit int) {
	stderr("WARNING: The signed/verified units feature is DEPRECATED and cannot be used.")
	return 2
}
