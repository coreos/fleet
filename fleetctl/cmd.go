package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
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
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/ssh"
	"github.com/coreos/fleet/unit"
	"github.com/coreos/fleet/version"
)

var out *tabwriter.Writer
var flagset *flag.FlagSet = flag.NewFlagSet("fleetctl", flag.ExitOnError)

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

func getRegistry(context *cli.Context) *registry.Registry {
	tun := getTunnelFlag()
	endpoint := getEndpointFlag()

	machines := []string{endpoint}
	client := etcd.NewClient(machines)

	if tun != "" {
		sshClient, err := ssh.NewSSHClient("core", tun)
		if err != nil {
			log.Fatalf("Failed initializing SSH client: %v", err)
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

func main() {
	app := cli.NewApp()
	app.Version = version.Version
	app.Name = "fleetctl"
	app.Usage = "fleetctl is a command line driven interface to the cluster wide CoreOS init system."

	app.Flags = []cli.Flag{
		cli.StringFlag{"endpoint", "http://127.0.0.1:4001", "Fleet Engine API endpoint (etcd)"},
		cli.StringFlag{"tunnel", "", "Establish an SSH tunnel through the provided address for communication with fleet and etcd."},
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
	}

	//flagset := flag.NewFlagSet("fleetctl", flag.ExitOnError)
	for _, flag := range app.Flags {
		flag.Apply(flagset)
	}

	flagset.Parse(os.Args[1:])

	globalconf.EnvPrefix="FLEETCTL_"
	globalconf.Register("fleetctl", flagset)
	gconf := globalconf.NewWithoutFile()
	gconf.ParseSet("", flagset)

	app.Run(os.Args)
}

func ellipsize(field string, n int) string {
	// When ellipsing a field, we want to display the first n
	// characters. We check for n+3 so we don't inadvertently
	// make fields with fewer than n+3 characters even longer
	// by adding unnecessary ellipses.
	if len(field) > n+3 {
		return fmt.Sprintf("%s...", field[0:n])
	} else {
		return field
	}
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
