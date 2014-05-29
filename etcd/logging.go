package etcd

import (
	"io/ioutil"
	"log"
	"os"

	goetcd "github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
)

func EnableDebugLogging() {
	goetcd.SetLogger(log.New(os.Stdout, "go-etcd", log.LstdFlags))
}

func DisableDebugLogging() {
	goetcd.SetLogger(log.New(ioutil.Discard, "go-etcd", log.LstdFlags))
}
