package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/ssh"
)

var (
	flagMachine            string
	flagUnit               string
	flagSSHAgentForwarding bool
	cmdSSH                 = &Command{
		Name:    "ssh",
		Summary: "Open interactive shell on a machine in the cluster",
		Usage:   "[-A|--forward-agent] [--machine|--unit] {MACHINE|UNIT}",
		Description: `Open an interactive shell on a specific machine in the cluster or on the machine 
where the specified unit is located.

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
		Run: runSSH,
	}
)

func init() {
	cmdSSH.Flags.StringVar(&flagMachine, "machine", "", "Open SSH connection to a specific machine.")
	cmdSSH.Flags.StringVar(&flagUnit, "unit", "", "Open SSH connection to machine running provided unit.")
	cmdSSH.Flags.BoolVar(&flagSSHAgentForwarding, "forward-agent", false, "Forward local ssh-agent to target machine.")
	cmdSSH.Flags.BoolVar(&flagSSHAgentForwarding, "A", false, "Shorthand for --forward-agent")
}

func runSSH(args []string) (exit int) {
	if flagUnit != "" && flagMachine != "" {
		stderr("Both machine and unit flags provided, please specify only one.")
		return 1
	}

	var err error
	var addr string

	switch {
	case flagMachine != "":
		addr, _, err = findAddressInMachineList(flagMachine)
	case flagUnit != "":
		addr, _, err = findAddressInRunningUnits(flagUnit)
	default:
		addr, err = globalMachineLookup(args)
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

	args = pkg.TrimToDashes(args)

	var sshClient *ssh.SSHForwardingClient
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient(globalFlags.SSHUsername, tun, addr, getChecker(), flagSSHAgentForwarding)
	} else {
		sshClient, err = ssh.NewSSHClient(globalFlags.SSHUsername, addr, getChecker(), flagSSHAgentForwarding)
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
			exit = 1
		}
	}
	return
}

func globalMachineLookup(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("one machine or unit must be provided")
	}

	lookup := args[0]

	machineAddr, machineOk, _ := findAddressInMachineList(lookup)
	unitAddr, unitOk, _ := findAddressInRunningUnits(lookup)

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

func findAddressInMachineList(lookup string) (string, bool, error) {
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

func findAddressInRunningUnits(name string) (string, bool, error) {
	name = unitNameMangle(name)
	u, err := cAPI.Unit(name)
	if err != nil {
		return "", false, err
	} else if u == nil {
		return "", false, fmt.Errorf("unit does not exist")
	} else if suToGlobal(*u) {
		return "", false, fmt.Errorf("global units unsupported")
	}

	m := cachedMachineState(u.MachineID)
	if m != nil && m.PublicIP != "" {
		return m.PublicIP, true, nil
	}

	return "", false, nil
}

// runCommand will attempt to run a command on a given machine. It will attempt
// to SSH to the machine if it is identified as being remote.
func runCommand(cmd string, machID string) (retcode int) {
	var err error
	if machine.IsLocalMachineID(machID) {
		err, retcode = runLocalCommand(cmd)
		if err != nil {
			stderr("Error running local command: %v", err)
		}
	} else {
		ms, err := machineState(machID)
		if err != nil || ms == nil {
			stderr("Error getting machine IP: %v", err)
		} else {
			err, retcode = runRemoteCommand(cmd, ms.PublicIP)
			if err != nil {
				stderr("Error running remote command: %v", err)
			}
		}
	}
	return
}

// runLocalCommand runs the given command locally and returns any error encountered and the exit code of the command
func runLocalCommand(cmd string) (error, int) {
	cmdSlice := strings.Split(cmd, " ")
	osCmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
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
func runRemoteCommand(cmd string, addr string) (err error, exit int) {
	var sshClient *ssh.SSHForwardingClient
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient(globalFlags.SSHUsername, tun, addr, getChecker(), false)
	} else {
		sshClient, err = ssh.NewSSHClient(globalFlags.SSHUsername, addr, getChecker(), false)
	}
	if err != nil {
		return err, -1
	}

	defer sshClient.Close()

	return ssh.Execute(sshClient, cmd)
}
