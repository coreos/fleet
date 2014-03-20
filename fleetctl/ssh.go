package main

import (
	"errors"
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
		Name:  "ssh",
		Usage: "Open interactive shell on a machine in the cluster",
		Description: `Open an interactive shell on a specific machine in the cluster or on the machine where the specified unit is located.

Open a shell on a machine:
fleetctl ssh 2444264c-eac2-4eff-a490-32d5e5e4af24

Open a shell from your laptop, to the machine running a specific unit, using a
cluster member as a bastion host:
fleetctl --tunnel 10.10.10.10 ssh foo.service

Tip: fleetctl tries to detect whether your first argument is a machine or a unit. To skip this check use the flags "-m" and "-u".

Pro-Tip: Create an alias for --tunnel:
Add "alias fleetctl=fleetctl --tunnel 10.10.10.10" to your bash profile.
Now you can run all fleet commands locally.`,
		Flags: []cli.Flag{
			cli.StringFlag{"machine, m", "", "Open SSH connection to a specific machine."},
			cli.StringFlag{"unit, u", "", "Open SSH connection to machine running provided unit."},
		},
		Action: sshAction,
	}
}

func sshAction(c *cli.Context) {
	unit := c.String("unit")
	machine := c.String("machine")

	if unit != "" && machine != "" {
		log.Fatal("Both flags, machine and unit provided, please specify only one")
	}

	args := c.Args()
	var err error
	var addr string

	switch {
	case machine != "":
		addr, _ = findAddressInMachineList(machine)
	case unit != "":
		addr, _ = findAddressInRunningUnits(unit)
	default:
		addr, err = globalMachineLookup(args)
		args = args[1:]
	}

	if err != nil {
		log.Fatal(err)
	}

	if addr == "" {
		log.Fatalf("Requested machine could not be found")
	}

	var sshClient *gossh.ClientConn
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker())
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker())
	}
	if err != nil {
		log.Fatal(err.Error())
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

			fmt.Print(string(bytes))
			if !prefix {
				fmt.Print("\n")
			}
		}

	} else {
		if err := ssh.Shell(sshClient); err != nil {
			log.Fatalf(err.Error())
		}
	}
}

func globalMachineLookup(args []string) (string, error) {
	if len(args) == 0 {
		log.Fatalf("Provide one machine or unit")
	}

	lookup := args[0]

	machineAddr, machineOk := findAddressInMachineList(lookup)
	unitAddr, unitOk := findAddressInRunningUnits(lookup)

	switch {
	case machineOk && unitOk:
		return "", errors.New(fmt.Sprintf("Ambiguous argument, both machine and unit found for `%s`.\nPlease use flag `-m` or `-u` to refine the search", lookup))
	case machineOk:
		return machineAddr, nil
	case unitOk:
		return unitAddr, nil
	}

	return "", nil
}

func findAddressInMachineList(lookup string) (string, bool) {
	states := registryCtl.GetActiveMachines()
	var match *machine.MachineState

	for i, _ := range states {
		machState := states[i]
		if !strings.HasPrefix(machState.BootID, lookup) {
			continue
		} else if match != nil {
			log.Fatalf("Found more than one Machine, be more specfic")
		}
		match = &machState
	}

	if match == nil {
		return "", false
	}

	return fmt.Sprintf("%s:22", match.PublicIP), true
}

func findAddressInRunningUnits(lookup string) (string, bool) {
	js := registryCtl.GetJobState(lookup)
	if js == nil {
		return "", false
	}
	return fmt.Sprintf("%s:22", js.MachineState.PublicIP), true
}
