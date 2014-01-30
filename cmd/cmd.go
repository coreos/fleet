package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/codegangsta/cli"
	"github.com/coreos/go-etcd/etcd"

	"github.com/coreos/coreinit/registry"
	"github.com/coreos/coreinit/ssh"
)

var out *tabwriter.Writer

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
}

func getRegistry(context *cli.Context) *registry.Registry {
	tun := context.GlobalString("tunnel")
	endpoint := context.GlobalString("endpoint")

	machines := []string{endpoint}
	client := etcd.NewClient(machines)

	if tun != "" {
		if !strings.Contains(tun, ":") {
			tun += ":22"
		}
		sshClient, err := ssh.NewSSHClient("core", tun)
		if err != nil {
			panic(err)
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
	app.Name = "corectl"
	app.Usage = "corectl is a command line driven interface to the cluster wide CoreOS init system."

	app.Flags = []cli.Flag{
		cli.StringFlag{"endpoint", "http://127.0.0.1:4001", "Coreinit Engine API endpoint (etcd)"},
		cli.StringFlag{"tunnel", "", "Establish an SSH tunnel through the provided address for all etcd communication."},
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
	}

	app.Run(os.Args)
}
