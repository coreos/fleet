package main

import (
	"fmt"
	"github.com/coreos/go-systemd/dbus"
)

func main() {
	script := []string{"/bin/sh", "-c",
		"while true; do echo goodbye world; sleep 1; done"}

	systemd := dbus.New()
	jobid, err := systemd.StartTransientUnit("hello.service",
		"replace",
		dbus.PropExecStart(script, false))
	fmt.Println(jobid, err)
}
