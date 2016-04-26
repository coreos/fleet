// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/codegangsta/cli"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/ssh"
)

func NewSSHCommend() cli.Command {
	return cli.Command{
		Name:      "ssh",
		Usage:     "Open interactive shell on a machine in the cluster",
		ArgsUsage: "[-A|--forward-agent] [--ssh-port=N] [--machine|--unit] {MACHINE|UNIT}",
		Description: `Open an interactive shell on a specific machine in the cluster or on the machine where the specified unit is located.

fleetctl tries to detect whether your first argument is a machine or a unit.
To skip this check use the --machine or --unit flags.

Open a shell on a machine:
       fleetctl ssh 2444264c-eac2-4eff-a490-32d5e5e4af24

Open a shell from your laptop, to the machine running a specific unit, using a
cluster member as a bastion host:
       fleetctl --tunnel 10.10.10.10 ssh foo.service

Open a shell on a machine and forward the authentication agent connection:
       fleetctl ssh --forward-agent 2444264c-eac2-4eff-a490-32d5e5e4af24


Tip: Create an alias for --tunnel.
       - Add "alias fleetctl=fleetctl --tunnel 10.10.10.10" to your bash profile.
       - Now you can run all fleet commands locally.

This command does not work with global units.`,
		Action: makeActionWrapper(runSSH),
		Flags: []cli.Flag{
			cli.StringFlag{Name: "machine", Value: "", Usage: "Open SSH connection to a specific machine."},
			cli.StringFlag{Name: "unit", Value: "", Usage: "Open SSH connection to machine running provided unit."},
			cli.BoolFlag{Name: "forward-agent, A", Usage: "forward local ssh-agent to target machine."},
			cli.IntFlag{Name: "ssh-port", Value: 22, Usage: "Connect to remote hosts over SSH using this TCP port."},
		},
	}
}

func runSSH(c *cli.Context, cAPI client.API) (exit int) {
	args := c.Args()
	if c.String("unit") != "" && c.String("machine") != "" {
		stderr("Both machine and unit flags provided, please specify only one.")
		return 1
	}

	var err error
	var addr string

	switch {
	case c.String("machine") != "":
		addr, _, err = findAddressInMachineList(c.String("machine"), cAPI)
	case c.String("unit") != "":
		addr, _, err = findAddressInRunningUnits(c.String("unit"), cAPI)
	default:
		addr, err = globalMachineLookup(args, cAPI)
		// trim machine/unit name from args
		if len(args) > 0 {
			args = args[1:]
		}
	}

	if err != nil {
		stderr("Unable to proceed: %v", err)
		return 1
	}

	if addr == "" {
		stderr("Could not determine address of machine.")
		return 1
	}

	addr = findSSHPort(addr, c)

	args = pkg.TrimToDashes(args)

	var sshClient *ssh.SSHForwardingClient
	timeout := getSSHTimeoutFlag(c)
	if tun := getTunnelFlag(c); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient(c.GlobalString("ssh-username"), tun, addr, getChecker(c), c.Bool("forward-agent"), timeout)
	} else {
		sshClient, err = ssh.NewSSHClient(c.GlobalString("ssh-username"), addr, getChecker(c), c.Bool("forward-agent"), timeout)
	}
	if err != nil {
		stderr("Failed building SSH client: %v", err)
		return 1
	}

	defer sshClient.Close()

	if len(args) > 0 {
		cmd := strings.Join(args, " ")
		err, exit = ssh.Execute(sshClient, cmd)
		if err != nil {
			stderr("Failed running command over SSH: %v", err)
		}
	} else {
		if err := ssh.Shell(sshClient); err != nil {
			stderr("Failed opening shell over SSH: %v", err)
			return 1
		}
	}

	return
}

func findSSHPort(addr string, c *cli.Context) string {
	if c.Int("ssh-port") != 22 && !strings.Contains(addr, ":") {
		return net.JoinHostPort(addr, strconv.Itoa(c.Int("ssh-port")))
	} else {
		return addr
	}
}

func globalMachineLookup(args []string, cAPI client.API) (string, error) {
	if len(args) == 0 {
		return "", errors.New("one machine or unit must be provided")
	}

	lookup := args[0]

	machineAddr, machineOk, _ := findAddressInMachineList(lookup, cAPI)
	unitAddr, unitOk, _ := findAddressInRunningUnits(lookup, cAPI)

	switch {
	case machineOk && unitOk:
		return "", fmt.Errorf("ambiguous argument, both machine and unit found for `%s`.\nPlease use flag `-m` or `-u` to refine the search", lookup)
	case machineOk:
		return machineAddr, nil
	case unitOk:
		return unitAddr, nil
	}

	return "", fmt.Errorf("could not find matching unit or machine")
}

func findAddressInMachineList(lookup string, cAPI client.API) (string, bool, error) {
	states, err := cAPI.Machines()
	if err != nil {
		return "", false, err
	}

	var match *machine.MachineState
	for i := range states {
		machState := states[i]
		if !strings.HasPrefix(machState.ID, lookup) {
			continue
		}

		if match != nil {
			return "", false, fmt.Errorf("found more than one machine")
		}

		match = &machState
	}

	if match == nil {
		return "", false, fmt.Errorf("machine does not exist")
	}

	return match.PublicIP, true, nil
}

func findAddressInRunningUnits(name string, cAPI client.API) (string, bool, error) {
	name = unitNameMangle(name)
	u, err := cAPI.Unit(name)
	if err != nil {
		return "", false, err
	} else if u == nil {
		return "", false, fmt.Errorf("unit does not exist")
	} else if suToGlobal(*u) {
		return "", false, fmt.Errorf("global units unsupported")
	}

	m := cachedMachineState(u.MachineID, cAPI)
	if m != nil && m.PublicIP != "" {
		return m.PublicIP, true, nil
	}

	return "", false, nil
}

// runCommand will attempt to run a command on a given machine. It will attempt
// to SSH to the machine if it is identified as being remote.
func runCommand(c *cli.Context, cAPI client.API, machID string, cmd string, args ...string) (retcode int) {
	var err error
	if machine.IsLocalMachineID(machID) {
		err, retcode = runLocalCommand(cmd, args...)
		if err != nil {
			stderr("Error running local command: %v", err)
		}
	} else {
		ms, err := machineState(machID, cAPI)
		if err != nil || ms == nil {
			stderr("Error getting machine IP: %v", err)
		} else {
			addr := findSSHPort(ms.PublicIP, c)
			err, retcode = runRemoteCommand(c, addr, cmd, args...)
			if err != nil {
				stderr("Error running remote command: %v", err)
			}
		}
	}
	return
}

// runLocalCommand runs the given command locally and returns any error encountered and the exit code of the command
func runLocalCommand(cmd string, args ...string) (error, int) {
	osCmd := exec.Command(cmd, args...)
	osCmd.Stderr = os.Stderr
	osCmd.Stdout = os.Stdout
	osCmd.Start()
	err := osCmd.Wait()
	if err != nil {
		// Get the command's exit status if we can
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return nil, status.ExitStatus()
			}
		}
		// Otherwise, generic command error
		return err, -1
	}
	return nil, 0
}

// runRemoteCommand runs the given command over SSH on the given IP, and returns
// any error encountered and the exit status of the command
func runRemoteCommand(c *cli.Context, addr string, cmd string, args ...string) (err error, exit int) {
	var sshClient *ssh.SSHForwardingClient
	timeout := getSSHTimeoutFlag(c)
	if tun := getTunnelFlag(c); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient(c.GlobalString("ssh-username"), tun, addr, getChecker(c), false, timeout)
	} else {
		sshClient, err = ssh.NewSSHClient(c.GlobalString("ssh-username"), addr, getChecker(c), false, timeout)
	}
	if err != nil {
		return err, -1
	}

	cmdargs := cmd
	for _, arg := range args {
		cmdargs += fmt.Sprintf(" %q", arg)
	}

	defer sshClient.Close()

	return ssh.Execute(sshClient, cmdargs)
}
