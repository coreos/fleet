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
	"strings"
	"text/tabwriter"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/fleet/third_party/github.com/rakyll/globalconf"

	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/ssh"
	"github.com/coreos/fleet/unit"
	"github.com/coreos/fleet/version"
)

var out *tabwriter.Writer
var flagset *flag.FlagSet = flag.NewFlagSet("fleetctl", flag.ExitOnError)
var registryCtl Registry

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
	cli.CommandHelpTemplate = `NAME:
   fleetctl {{.Name}} - {{.Usage}}

DESCRIPTION:
   {{.Description}}

OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}
`
}

func getRegistry() *registry.Registry {
	tun := getTunnelFlag()
	endpoint := getEndpointFlag()

	machines := []string{endpoint}
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

func getChecker() *ssh.HostKeyChecker {
	if !(*flagset.Lookup("strict-host-key-checking")).Value.(flag.Getter).Get().(bool) {
		return nil
	}

	knownHostsFile := (*flagset.Lookup("known-hosts-file")).Value.(flag.Getter).Get().(string)
	keyFile := ssh.NewHostKeyFile(knownHostsFile)
	return ssh.NewHostKeyChecker(keyFile, askToTrustHost, nil)
}

func main() {
	app := cli.NewApp()
	app.Name = "fleetctl"
	app.Usage = "fleetctl is a command-line interface to fleet, the cluster-wide CoreOS init system."
	app.Version = version.Version

	app.Flags = []cli.Flag{
		cli.StringFlag{"endpoint", "http://127.0.0.1:4001", "Fleet Engine API endpoint (etcd)"},
		cli.StringFlag{"tunnel", "", "Establish an SSH tunnel through the provided address for communication with fleet and etcd."},
		cli.BoolTFlag{"strict-host-key-checking", "Verify host keys presented by remote machines before initiating SSH connections."},
		cli.StringFlag{"known-hosts-file", ssh.DefaultKnownHostsFile, "File used to store remote machine fingerprints. Ignored if strict host key checking is disabled."},
		cli.BoolFlag{"debug", "Print out more debug information."},
	}

	app.Commands = []cli.Command{
		newListUnitsCommand(),
		newSubmitUnitCommand(),
		newDestroyUnitCommand(),
		newStartUnitCommand(),
		newStopUnitCommand(),
		newStatusUnitsCommand(),
		newCatUnitCommand(),
		newListMachinesCommand(),
		newJournalCommand(),
		newSSHCommand(),
		newVerifyUnitCommand(),
		newDebugInfoCommand(),
	}

	for _, f := range app.Flags {
		f.Apply(flagset)
	}

	flagset.Bool("version", false, "Print the version and exit")
	flagset.Bool("v", false, "Print the version and exit")

	flagset.Parse(os.Args[1:])

	setupGlog()

	if (*flagset.Lookup("version")).Value.(flag.Getter).Get().(bool) {
		fmt.Println("fleetctl version", version.Version)
		os.Exit(0)
	}

	globalconf.Register("fleetctl", flagset)
	opts := globalconf.Options{EnvPrefix: "FLEETCTL_"}
	gconf, _ := globalconf.NewWithOptions(&opts)
	gconf.ParseSet("", flagset)

	registryCtl = NewRegistry(getRegistry())
	app.Run(os.Args)
}

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
	tun := (*flagset.Lookup("tunnel")).Value.(flag.Getter).Get().(string)
	if tun != "" && !strings.Contains(tun, ":") {
		tun += ":22"
	}
	return tun
}

func getEndpointFlag() string {
	return (*flagset.Lookup("endpoint")).Value.(flag.Getter).Get().(string)
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

	// Send it to stderr and don't pollute stdout
	fmt.Fprintf(os.Stderr, "The authenticity of host '%v' can't be established.\n%v key fingerprint is %v.\nAre you sure you want to continue connecting (yes/no)? ", addr, algo, fingerprint)
	fmt.Scanf("%s\n", &ans)

	ans = strings.ToLower(ans)
	if ans != "yes" && ans != "y" {
		return false
	}

	return true
}

// setupGlog sets the configuration for glog.
// Check -debug flag, set the flags used by glog in the default set of
// command-line flags, and reparse flagset to activate them.
func setupGlog() {
	verbosity := "0"
	if (*flagset.Lookup("debug")).Value.(flag.Getter).Get().(bool) {
		verbosity = "1"
	}

	err := flag.CommandLine.Lookup("v").Value.Set(verbosity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to apply verbosity to flag.v: %v\n", err)
	}

	err = flag.CommandLine.Lookup("logtostderr").Value.Set("true")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set flag.logtostderr to true: %v\n", err)
	}

	flagset.Parse(os.Args[1:])
}
