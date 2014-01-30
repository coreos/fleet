package main

import (
	"bufio"
	"fmt"
	"log"
	"syscall"

	"github.com/codegangsta/cli"
)

func newJournalCommand() cli.Command {
	return cli.Command{
		Name:        "journal",
		Usage:       "Interact with journalctl.",
		Description: "",
		Action:      journalAction,
		Flags: []cli.Flag{
			cli.IntFlag{"lines, n", 10, "Number of log lines to return."},
		},
	}
}

func journalAction(c *cli.Context) {
	if len(c.Args()) != 1 {
		fmt.Println("One unit file must be provided.")
		syscall.Exit(1)
	}
	jobName := c.Args()[0]

	r := getRegistry(c)
	js := r.GetJobState(jobName)

	if js == nil {
		fmt.Println("Unit does not appear to be running")
		syscall.Exit(1)
	}

	tunnel, err := ssh("core", fmt.Sprintf("%s:22", js.Machine.PublicIP))
	if err != nil {
		log.Fatalf("Unable to SSH to coreinit host: %s", err.Error())
	}

	stdout, _ := tunnel.StdoutPipe()
	bstdout := bufio.NewReader(stdout)
	cmd := fmt.Sprintf("journalctl -u %s --no-pager -l -n %d", jobName, c.Int("lines"))

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
