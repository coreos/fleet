package main

import (
	"fmt"
	"log"
	"strings"

	gossh "github.com/coreos/fleet/third_party/code.google.com/p/go.crypto/ssh"
	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/ssh"
)

func newSSHCommand() cli.Command {
	return cli.Command{
		Name:	"ssh",
		Usage:	"Open interactive shell on a machine in the cluster",
		Description: `Open an interactive shell on a specific machine in the cluster or on the machine where the specified unit is located.

Open a shell on a machine:
fleetctl ssh 2444264c-eac2-4eff-a490-32d5e5e4af24

Open a shell from your laptop, to the machine running a specific unit, using a
cluster member as a bastion host:
fleetctl --tunnel 10.10.10.10 ssh -u foo.service

Pro-Tip: Create an alias for --tunnel:
Add "alias fleetctl=fleetctl --tunnel 10.10.10.10" to your bash profile.
Now you can run all fleet commands locally.`,
		Action:	sshAction,
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
		lookup := args[0]
		args = args[1:]
		machines := r.GetActiveMachines()
		var match *machine.Machine
		for i, _ := range machines {
			mach := machines[i]
			if !strings.HasPrefix(mach.BootId, lookup) {
				continue
			} else if match != nil {
				log.Fatalf("Found more than one Machine, be more specfic")
			}
			match = &mach
		}

		if match == nil {
			log.Fatalf("Could not find provided Machine")
		}

		addr = fmt.Sprintf("%s:22", match.PublicIP)
	} else {
		js := r.GetJobState(unit)
		if js == nil {
			log.Fatalf("Requested unit %s does not appear to be running", unit)
		}
		addr = fmt.Sprintf("%s:22", js.Machine.PublicIP)
	}

	var err error
	var sshClient *gossh.ClientConn
	if tun := getTunnelFlag(c); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr)
	}
	if err != nil {
		log.Fatalf("Unable to establish SSH connection: %v", err)
		return
	}

	defer sshClient.Close()

	if len(args) > 0 {
		cmd := strings.Join(args, " ")
		stdout, err := ssh.Execute(sshClient, cmd)
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
		if err := ssh.Shell(sshClient); err != nil {
			log.Fatalf(err.Error())
		}
	}
}
