package registry

import (
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/third_party/github.com/coreos/go-etcd/etcd"
)

func freePort() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}

	return port, nil
}

func localAddr(port string) string {
	return "127.0.0.1:" + port
}

// Assume etcd binary is in PATH
func startEtcd(addr string, t *testing.T) (*exec.Cmd, error) {
	dataDir, err := ioutil.TempDir("", "registry_machine_test")
	if err != nil {
		return nil, err
	}

	port, err := freePort()
	if err != nil {
		t.Fatalf("couldn't get a free port: %v", err)
	}

	peerAddr := localAddr(port)

	cmd := exec.Command("etcd", "-name=testing", "-data-dir="+dataDir, "-addr="+addr, "-peer-addr="+peerAddr)

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	// wait for etcd to come up
	time.Sleep(5 * time.Second)

	return cmd, nil
}

func stopEtcd(cmd *exec.Cmd) {
	cmd.Process.Kill()
}

func startTestRegistry(t *testing.T) (*Registry, *exec.Cmd) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("couldn't get a free port: %v", err)
	}

	etcdAddr := localAddr(port)

	etcdCmd, err := startEtcd(etcdAddr, t)
	if err != nil {
		t.Fatalf("couldn't start etcd: %v", err)
	}
	r := New(etcd.NewClient([]string{"http://" + etcdAddr}))
	return r, etcdCmd
}

func checkSpecs(expected, given *machine.MachineSpec, t *testing.T) {
	if !specsEqual(expected, given) {
		t.Errorf("expected %+v, got %+v", *expected, *given)
	}
}

func specsEqual(expected, given *machine.MachineSpec) bool {
	if expected.Cores != given.Cores {
		return false
	}

	if expected.Memory != given.Memory {
		return false
	}

	if expected.DiskSpace != given.DiskSpace {
		return false
	}
	return true
}

func TestGetSetMachineSpec(t *testing.T) {
	r, etcdCmd := startTestRegistry(t)
	defer stopEtcd(etcdCmd)

	spec := machine.MachineSpec{
		Cores:     10,
		Memory:    1024,
		DiskSpace: 1024,
	}

	err := r.SetMachineSpec("host1", spec)
	if err != nil {
		t.Errorf("error setting spec: %v", err)
	}

	rspec, err := r.GetMachineSpec("host1")
	if err != nil {
		t.Errorf("error getting spec: %v", err)
	}

	checkSpecs(&spec, rspec, t)
}

func TestGetNonExistentMachineSpec(t *testing.T) {
	r, etcdCmd := startTestRegistry(t)
	defer stopEtcd(etcdCmd)

	rspec, err := r.GetMachineSpec("foobar")
	if err != nil {
		t.Errorf("error getting spec: %v", err)
	}
	if rspec != nil {
		t.Errorf("expected a nil machine spec")
	}
}

func TestGetAllMachineSpecs(t *testing.T) {
	r, etcdCmd := startTestRegistry(t)
	defer stopEtcd(etcdCmd)

	specs := make(map[string]machine.MachineSpec)

	for i := 0; i < 100; i++ {
		spec := machine.MachineSpec{
			Cores:     i,
			Memory:    i,
			DiskSpace: i,
		}
		bootID := fmt.Sprintf("host%d", i)
		specs[bootID] = spec

		err := r.SetMachineSpec(bootID, spec)
		if err != nil {
			t.Errorf("error setting spec: %v", err)
		}
	}

	rspecs, err := r.GetMachineSpecs()
	if err != nil {
		t.Errorf("error getting specs: %v", err)
	}

	if len(specs) != len(rspecs) {
		t.Errorf("expected %d specs, got %d", len(specs), len(rspecs))
	}

	for k, v := range specs {
		expected := v
		given := rspecs[k]
		checkSpecs(&expected, &given, t)
	}
}
