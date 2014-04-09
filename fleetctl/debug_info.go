package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

var cmdDebugInfo = &Command{
	Name:        "debug-info",
	Summary:     "Print out debug information",
	Description: `Lists all values stored in etcd, which could reflect the status of the cluster comprehensively`,
	Run:         runDebugInfo,
}

func runDebugInfo(args []string) (exit int) {
	info, err := registryCtl.GetDebugInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed communicating with etcd: %v\n", err)
		return 1
	}

	fmt.Println("All fleet entries in etcd service:")
	buf := new(bytes.Buffer)
	if err = json.Indent(buf, []byte(info), "", "\t"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed indenting json: %v\n", err)
		return 1
	}
	fmt.Println(buf.String())
	return 0
}
