package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/ssh"
)

func newSSHCommand() cli.Command {
	return cli.Command{
		Name:   "ssh",
		Usage:  "Open interactive shell on a machine in the cluster",
		Action: sshAction,
		Flags: []cli.Flag{
			cli.StringFlag{"unit, u", "", "Open SSH connection to machine running provided unit."},
		},
	}
}

func sshAction(c *cli.Context) {
	r := getRegistry(c)

	args := c.Args()
	unit := c.String("unit")
	if len(args) == 0 && unit == "" {
		log.Fatalf("Provide one machine or unit")
	}

	var addr string
	if unit == "" {
		m := args[0]
		args = args[1:]

		ms := r.GetMachineState(m)
		if ms == nil {
			log.Fatalf("Machine %s could not be found", m)
		}
		addr = fmt.Sprintf("%s:22", ms.PublicIP)
	} else {
		js := r.GetJobState(unit)
		if js == nil {
			log.Fatalf("Requested unit %s does not appear to be running", unit)
		}
		addr = fmt.Sprintf("%s:22", js.Machine.PublicIP)
	}

	if len(args) > 0 {
		cmd := strings.Join(args, " ")
		stdout, err := ssh.Execute("core", addr, cmd)
		if err != nil {
			log.Fatalf("Unable to run command over SSH: %s", err.Error())
		}

		for {
			bytes, prefix, err := stdout.ReadLine()
			if err != nil {
				break
			}

			print(string(bytes))
			if !prefix {
				print("\n")
			}
		}

	} else {
		if err := ssh.Shell("core", addr); err != nil {
			log.Fatalf(err.Error())
		}
	}
}
