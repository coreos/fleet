package main

import (
	"fmt"
	"github.com/coreos/go-systemd/dbus"
)

func main() {
	systemd := dbus.New()
	jobid, err := systemd.StartTransientUnit("hello.service", "replace", dbus.PropExecStart([]string{"/bin/sh", "-c", "echo hello world"}, false))
	fmt.Println(jobid, err)
}
