package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/ssh"
	"github.com/coreos/fleet/unit"
)

const (
	cliName        = "fleetctl"
	cliDescription = "fleetctl is a command-line interface to fleet, the cluster-wide CoreOS init system."
)

var (
	out           *tabwriter.Writer
	globalFlagset *flag.FlagSet = flag.NewFlagSet("fleetctl", flag.ExitOnError)

	// set of top-level commands
	commands []*Command

	// global Registry used by commands
	registryCtl Registry

	// flags used by all commands
	globalFlags = struct {
		Debug                 bool
		Verbosity             int
		Version               bool
		Endpoint              string
		KnownHostsFile        string
		StrictHostKeyChecking bool
		Tunnel                string
	}{}

	// flags used by multiple commands
	sharedFlags = struct {
		Sign     bool
		Full     bool
		NoLegend bool
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information.")
	globalFlagset.BoolVar(&globalFlags.Version, "version", false, "Print the version and exit")
	globalFlagset.IntVar(&globalFlags.Verbosity, "verbosity", 0, "Log at a specified level")
	globalFlagset.StringVar(&globalFlags.Endpoint, "endpoint", "http://127.0.0.1:4001", "Fleet Engine API endpoint (etcd)")
	globalFlagset.StringVar(&globalFlags.KnownHostsFile, "known-hosts-file", ssh.DefaultKnownHostsFile, "File used to store remote machine fingerprints. Ignored if strict host key checking is disabled.")
	globalFlagset.BoolVar(&globalFlags.StrictHostKeyChecking, "strict-host-key-checking", true, "Verify host keys presented by remote machines before initiating SSH connections.")
	globalFlagset.StringVar(&globalFlags.Tunnel, "tunnel", "", "Establish an SSH tunnel through the provided address for communication with fleet and etcd.")
}

type Command struct {
	Name        string       // Name of the Command and the string to use to invoke it
	Summary     string       // One-sentence summary of what the Command does
	Usage       string       // Usage options/arguments
	Description string       // Detailed description of command
	Flags       flag.FlagSet // Set of flags associated with this command

	Run func(args []string) int // Run a command with the given arguments, return exit status

}

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
	commands = []*Command{
		cmdCatUnit,
		cmdDebugInfo,
		cmdDestroyUnit,
		cmdHelp,
		cmdJournal,
		cmdListMachines,
		cmdListUnits,
		cmdSSH,
		cmdStartUnit,
		cmdStatusUnits,
		cmdStopUnit,
		cmdSubmitUnit,
		cmdVerifyUnit,
		cmdVersion,
	}
}

func getAllFlags() (flags []*flag.Flag) {
	return getFlags(globalFlagset)
}

func getFlags(flagset *flag.FlagSet) (flags []*flag.Flag) {
	flags = make([]*flag.Flag, 0)
	flagset.VisitAll(func(f *flag.Flag) {
		flags = append(flags, f)
	})
	return
}

func main() {
	// parse global arguments
	globalFlagset.Parse(os.Args[1:])

	var args = globalFlagset.Args()

	getFlagsFromEnv(cliName, globalFlagset)

	// configure glog, which uses the global command line options
	if globalFlags.Debug {
		flag.CommandLine.Lookup("v").Value.Set(strconv.Itoa(1))
	}
	flag.CommandLine.Lookup("logtostderr").Value.Set("true")

	// no command specified - trigger help
	if len(args) < 1 {
		args = append(args, "help")
	}

	// deal specially with --version
	if globalFlags.Version {
		args[0] = "version"
	}

	var cmd *Command

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
				fmt.Println(err.Error())
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		fmt.Printf("%v: unknown subcommand: %q\n", cliName, args[0])
		fmt.Printf("Run '%v help' for usage.\n", cliName)
		os.Exit(2)
	}

	// TODO(jonboulle): increase cleverness of registry initialization
	if cmd.Name != "help" && cmd.Name != "version" {
		registryCtl = NewRegistry(getRegistry())
	}

	os.Exit(cmd.Run(cmd.Flags.Args()))

}

// getFlagsFromEnv parses all registered flags in the given flagset,
// and if they are not already set it attempts to set their values from
// environment variables. Environment variables take the name of the flag but
// are UPPERCASE, have the given prefix, and any dashes are replaced by
// underscores - for example: some-flag => PREFIX_SOME_FLAG
func getFlagsFromEnv(prefix string, fs *flag.FlagSet) {
	alreadySet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		alreadySet[f.Name] = true
	})
	fs.VisitAll(func(f *flag.Flag) {
		if !alreadySet[f.Name] {
			key := strings.ToUpper(prefix + "_" + strings.Replace(f.Name, "-", "_", -1))
			val := os.Getenv(key)
			if val != "" {
				fs.Set(f.Name, val)
			}
		}

	})
}

// getRegistry initializes a connection to the Registry
func getRegistry() *registry.Registry {
	tun := getTunnelFlag()

	machines := []string{globalFlags.Endpoint}
	client := etcd.NewClient(machines)

	if tun != "" {
		sshClient, err := ssh.NewSSHClient("core", tun, getChecker(), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed initializing SSH client: %v\n", err)
			os.Exit(1)
		}

		dial := func(network, addr string) (net.Conn, error) {
			tcpaddr, err := net.ResolveTCPAddr(network, addr)
			if err != nil {
				return nil, err
			}
			return sshClient.DialTCP(network, nil, tcpaddr)
		}

		tr := http.Transport{
			Dial: dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		client.SetTransport(&tr)
	}

	return registry.New(client)
}

// getChecker creates and returns a HostKeyChecker, or nil if any error is encountered
func getChecker() *ssh.HostKeyChecker {
	if !globalFlags.StrictHostKeyChecking {
		return nil
	}

	keyFile := ssh.NewHostKeyFile(strconv.FormatBool(globalFlags.StrictHostKeyChecking))
	return ssh.NewHostKeyChecker(keyFile, askToTrustHost, nil)
}

// getJobPayloadFromFile attempts to load a Job from a given filename
// It returns the Job or nil, and any error encountered
func getJobPayloadFromFile(file string) (*job.JobPayload, error) {
	out, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	unitFile := unit.NewSystemdUnitFile(string(out))

	name := path.Base(file)
	payload := job.NewJobPayload(name, *unitFile)

	return payload, nil
}

func getTunnelFlag() string {
	tun := globalFlags.Tunnel
	if tun != "" && !strings.Contains(tun, ":") {
		tun += ":22"
	}
	return tun
}

func machineBootIDLegend(ms machine.MachineState, full bool) string {
	legend := ms.BootID
	if !full {
		legend = fmt.Sprintf("%s...", ms.ShortBootID())
	}
	return legend
}

func machineFullLegend(ms machine.MachineState, full bool) string {
	legend := machineBootIDLegend(ms, full)
	if len(ms.PublicIP) > 0 {
		legend = fmt.Sprintf("%s/%s", legend, ms.PublicIP)
	}
	return legend
}

func askToTrustHost(addr, algo, fingerprint string) bool {
	var ans string

	fmt.Fprintf(os.Stderr, "The authenticity of host '%v' can't be established.\n%v key fingerprint is %v.\nAre you sure you want to continue connecting (yes/no)? ", addr, algo, fingerprint)
	fmt.Scanf("%s\n", &ans)

	ans = strings.ToLower(ans)
	if ans != "yes" && ans != "y" {
		return false
	}

	return true
}
