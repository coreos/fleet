package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/coreos/fleet/third_party/github.com/codegangsta/cli"
)

func newDebugInfoCommand() cli.Command {
	return cli.Command{
		Name:  "debug-info",
		Usage: "Print out debug information",
		Description: `Lists all values stored in etcd, which could reflect the status of the cluster comprehensively`,
		Action: debugInfoAction,
	}
}

func debugInfoAction(c *cli.Context) {
	info, err := registryCtl.GetDebugInfo()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Get response from etcd error:", err)
		return
	}

	fmt.Println("All fleet entries in etcd service:")
	buf := new(bytes.Buffer)
	if err = json.Indent(buf, []byte(info), "", "\t"); err != nil {
		return
	}
	fmt.Println(buf.String())
}
