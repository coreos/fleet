package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/coreos/fleet/Godeps/_workspace/src/github.com/golang/glog"
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
	- Now you can run all fleet commands locally.`,
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
		fmt.Fprintln(os.Stderr, "Both machine and unit flags provided, please specify only one.")
		return 1
	}

	var err error
	var addr string

	switch {
	case flagMachine != "":
		addr, _ = findAddressInMachineList(flagMachine)
	case flagUnit != "":
		addr, _ = findAddressInRunningUnits(flagUnit)
	default:
		addr, err = globalMachineLookup(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// trim machine/unit name from args
		if len(args) > 0 {
			args = args[1:]
		}
	}

	if addr == "" {
		fmt.Fprintln(os.Stderr, "Requested machine could not be found.")
		return 1
	}

	args = pkg.TrimToDashes(args)

	var sshClient *ssh.SSHForwardingClient
	if tun := getTunnelFlag(); tun != "" {
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker(), flagSSHAgentForwarding)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker(), flagSSHAgentForwarding)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed building SSH client: %v\n", err)
		return 1
	}

	defer sshClient.Close()

	if len(args) > 0 {
		cmd := strings.Join(args, " ")
		err, exit = ssh.Execute(sshClient, cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed running command over SSH: %v\n", err)
		}
	} else {
		if err := ssh.Shell(sshClient); err != nil {
			fmt.Fprintf(os.Stderr, "Failed opening shell over SSH: %v\n", err)
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

	machineAddr, machineOk := findAddressInMachineList(lookup)
	unitAddr, unitOk := findAddressInRunningUnits(lookup)

	switch {
	case machineOk && unitOk:
		return "", fmt.Errorf("ambiguous argument, both machine and unit found for `%s`.\nPlease use flag `-m` or `-u` to refine the search", lookup)
	case machineOk:
		return machineAddr, nil
	case unitOk:
		return unitAddr, nil
	}

	return "", nil
}

func findAddressInMachineList(lookup string) (string, bool) {
	states, err := cAPI.Machines()
	if err != nil {
		log.V(1).Infof("Unable to retrieve list of active machines from the Registry: %v", err)
		return "", false
	}

	var match *machine.MachineState

	for i := range states {
		machState := states[i]
		if !strings.HasPrefix(machState.ID, lookup) {
			continue
		} else if match != nil {
			fmt.Fprintln(os.Stderr, "Found more than one Machine, be more specific.")
			os.Exit(1)
		}
		match = &machState
	}

	if match == nil {
		return "", false
	}
	return match.PublicIP, true
}

func findAddressInRunningUnits(name string) (string, bool) {
	name = unitNameMangle(name)
	u, err := cAPI.Unit(name)
	if err != nil {
		log.V(1).Infof("Unable to retrieve Unit(%s) from Repository: %v", name, err)
	}
	if u == nil {
		return "", false
	}
	m := cachedMachineState(u.Machine)
	if m != nil && m.PublicIP != "" {
		return m.PublicIP, true
	}
	return "", false
}

// runCommand will attempt to run a command on a given machine. It will attempt
// to SSH to the machine if it is identified as being remote.
func runCommand(cmd string, machID string) (retcode int) {
	var err error
	if machine.IsLocalMachineID(machID) {
		err, retcode = runLocalCommand(cmd)
		if err != nil {
			fmt.Printf("Error running local command: %v\n", err)
		}
	} else {
		ms, err := machineState(machID)
		if err != nil || ms == nil {
			fmt.Printf("Error getting machine IP: %v\n", err)
		} else {
			err, retcode = runRemoteCommand(cmd, ms.PublicIP)
			if err != nil {
				fmt.Printf("Error running remote command: %v\n", err)
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
		sshClient, err = ssh.NewTunnelledSSHClient("core", tun, addr, getChecker(), false)
	} else {
		sshClient, err = ssh.NewSSHClient("core", addr, getChecker(), false)
	}
	if err != nil {
		return err, -1
	}

	defer sshClient.Close()

	return ssh.Execute(sshClient, cmd)
}
