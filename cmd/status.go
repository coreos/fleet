package main

import (
	"bufio"
	"fmt"
	"log"
	"path"

	"github.com/codegangsta/cli"

	"github.com/coreos/coreinit/registry"
)

func newStatusUnitsCommand() cli.Command {
	return cli.Command{
		Name:        "status",
		Usage:       "Fetch the status of one or more units",
		Description: ``,
		Action:      statusUnitsAction,
	}
}

func statusUnitsAction(c *cli.Context) {
	r := getRegistry(c)

	for i, v := range c.Args() {
		// This extra newline here to match systemctl status output
		if i != 0 {
			fmt.Printf("\n")
		}

		name := path.Base(v)
		printUnitStatus(r, name)
	}
}

func printUnitStatus(r *registry.Registry, unitName string) {
	j := r.GetJob(unitName)
	if j == nil {
		return
	}

	js := r.GetJobState(j)

	if js == nil {
		fmt.Println("%s does not appear to be running", unitName)
	}

	tunnel, err := ssh("core", fmt.Sprintf("%s:22", js.Machine.PublicIP))
	if err != nil {
		log.Fatalf("Unable to SSH to coreinit host: %s", err.Error())
	}

	stdout, _ := tunnel.StdoutPipe()
	bstdout := bufio.NewReader(stdout)
	cmd := fmt.Sprintf("systemctl status -l %s", unitName)

	tunnel.Start(cmd)
	go tunnel.Wait()

	for true {
		bytes, prefix, err := bstdout.ReadLine()
		if err != nil {
			break
		}

		print(string(bytes))
		if !prefix {
			print("\n")
		}
	}
}
