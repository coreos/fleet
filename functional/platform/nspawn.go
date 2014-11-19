/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package platform

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/go-systemd/dbus"

	"github.com/coreos/fleet/functional/util"
)

const (
	fleetAPIPort = 54728
)

var fleetdBinPath string

func init() {
	fleetdBinPath = os.Getenv("FLEETD_BIN")
	if fleetdBinPath == "" {
		fmt.Println("FLEETD_BIN environment variable must be set")
		os.Exit(1)
	} else if _, err := os.Stat(fleetdBinPath); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	// sanity check etcd availability
	cmd := exec.Command("etcdctl", "ls")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Unable to access etcd: %v\n", err)
		fmt.Println(string(out))
		os.Exit(1)
	}
}

type nspawnMember struct {
	id  string
	ip  string
	pid int
}

func (m *nspawnMember) ID() string {
	return m.id
}

func (m *nspawnMember) IP() string {
	return m.ip
}

type nspawnCluster struct {
	name    string
	members map[string]*nspawnMember
}

func (nc *nspawnCluster) keyspace() string {
	// TODO(jonboulle): generate this dynamically with atomic in order keys?
	return fmt.Sprintf("/fleet_functional/%s", nc.name)
}

func (nc *nspawnCluster) Fleetctl(args ...string) (string, string, error) {
	args = append([]string{"--etcd-key-prefix=" + nc.keyspace()}, args...)
	return util.RunFleetctl(args...)
}

func (nc *nspawnCluster) FleetctlWithInput(input string, args ...string) (string, string, error) {
	args = append([]string{"--etcd-key-prefix=" + nc.keyspace()}, args...)
	return util.RunFleetctlWithInput(input, args...)
}

func (nc *nspawnCluster) WaitForNMachines(count int) ([]string, error) {
	return util.WaitForNMachines(nc.Fleetctl, count)
}

func (nc *nspawnCluster) WaitForNActiveUnits(count int) (map[string][]util.UnitState, error) {
	return util.WaitForNActiveUnits(nc.Fleetctl, count)
}

func (nc *nspawnCluster) prepCluster() (err error) {
	baseDir := path.Join(os.TempDir(), nc.name)
	_, _, err = run(fmt.Sprintf("mkdir -p %s", baseDir))
	if err != nil {
		return
	}

	stdout, _, err := run("brctl show")
	if err != nil {
		log.Printf("Failed enumerating bridges: %v", err)
		return
	}

	if !strings.Contains(stdout, "fleet0") {
		_, _, err = run("brctl addbr fleet0")
		if err != nil {
			log.Printf("Failed adding fleet0 bridge: %v", err)
			return
		}
	} else {
		log.Printf("Bridge fleet0 already exists")
	}

	stdout, _, err = run("ip addr list fleet0")
	if err != nil {
		log.Printf("Failed listing fleet0 addresses: %v", err)
		return
	}

	if !strings.Contains(stdout, "172.17.0.1/16") {
		_, _, err = run("ip addr add 172.17.0.1/16 dev fleet0")
		if err != nil {
			log.Printf("Failed adding 172.17.0.1/16 to fleet0: %v", err)
			return
		}
	}

	_, _, err = run("ip link set fleet0 up")
	if err != nil {
		log.Printf("Failed bringing up fleet0 bridge: %v", err)
		return
	}

	return nil
}

func (nc *nspawnCluster) prepFleet(dir, ip, sshKeySrc, fleetdBinSrc string, cfg MachineConfig) error {
	cmd := fmt.Sprintf("mkdir -p %s/opt/fleet", dir)
	if _, _, err := run(cmd); err != nil {
		return err
	}

	relSSHKeyDst := path.Join("opt", "fleet", "id_rsa.pub")
	sshKeyDst := path.Join(dir, relSSHKeyDst)
	if err := copyFile(sshKeySrc, sshKeyDst, 0644); err != nil {
		return err
	}

	fleetdBinDst := path.Join(dir, "opt", "fleet", "fleetd")
	if err := copyFile(fleetdBinSrc, fleetdBinDst, 0755); err != nil {
		return err
	}

	cfgTmpl := `verbosity=2
etcd_servers=["http://172.17.0.1:4001"]	
etcd_key_prefix=%s
public_ip=%s
authorized_keys_file=%s
`
	cfgContents := fmt.Sprintf(cfgTmpl, nc.keyspace(), ip, relSSHKeyDst)
	cfgPath := path.Join(dir, "opt", "fleet", "fleet.conf")
	if err := ioutil.WriteFile(cfgPath, []byte(cfgContents), 0644); err != nil {
		return err
	}

	socketContents := fmt.Sprintf("[Socket]\nListenStream=%d\n", fleetAPIPort)
	socketPath := path.Join(dir, "opt", "fleet", "fleet.socket")
	if err := ioutil.WriteFile(socketPath, []byte(socketContents), 0644); err != nil {
		return err
	}

	serviceContents := `[Service]
ExecStart=/opt/fleet/fleetd -config /opt/fleet/fleet.conf
`
	servicePath := path.Join(dir, "opt", "fleet", "fleet.service")
	if err := ioutil.WriteFile(servicePath, []byte(serviceContents), 0644); err != nil {
		return err
	}

	return nil
}

func (nc *nspawnCluster) Members() []string {
	names := make([]string, 0)
	for member := range nc.members {
		names = append(names, member)
	}
	return names
}

func (nc *nspawnCluster) MemberCommand(member string, args ...string) (string, error) {
	ip := nc.members[member].ip
	baseArgs := []string{"-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no", fmt.Sprintf("core@%s", ip)}
	args = append(baseArgs, args...)
	log.Printf("ssh %s", strings.Join(args, " "))
	var stdoutBytes bytes.Buffer
	cmd := exec.Command("ssh", args...)
	cmd.Stdout = &stdoutBytes
	err := cmd.Run()
	return stdoutBytes.String(), err
}

func (nc *nspawnCluster) findUsableIP() (string, error) {
	base := 100
ip:
	for octet := base; octet < 256; octet++ {
		ip := fmt.Sprintf("172.17.1.%d", octet)
		for _, member := range nc.members {
			if ip == member.ip {
				continue ip
			}
		}
		return ip, nil
	}
	return "", errors.New("unable to find unused IP address")
}

func (nc *nspawnCluster) CreateMember(name string, cfg MachineConfig) (m Member, err error) {
	log.Printf("Creating nspawn machine %s in cluster %s", name, nc.name)

	nm := &nspawnMember{id: name}
	nc.members[name] = nm
	m = nm

	nm.ip, err = nc.findUsableIP()
	if err != nil {
		return
	}

	basedir := path.Join(os.TempDir(), nc.name)
	fsdir := path.Join(basedir, name, "fs")
	cmds := []string{
		// set up directory for fleet service
		fmt.Sprintf("mkdir -p %s/etc/systemd/system", fsdir),

		// minimum requirements for running systemd/coreos in a container
		fmt.Sprintf("mkdir -p %s/usr", fsdir),
		fmt.Sprintf("cp /etc/os-release %s/etc", fsdir),
		fmt.Sprintf("ln -s /proc/self/mounts %s/etc/mtab", fsdir),
		fmt.Sprintf("ln -s usr/lib64 %s/lib64", fsdir),
		fmt.Sprintf("ln -s lib64 %s/lib", fsdir),
		fmt.Sprintf("ln -s usr/bin %s/bin", fsdir),
		fmt.Sprintf("ln -s usr/sbin %s/sbin", fsdir),
		fmt.Sprintf("mkdir -p %s/home/core", fsdir),
		fmt.Sprintf("chown core:core %s/home/core", fsdir),

		// We don't need this, and it's slow, so mask it
		fmt.Sprintf("ln -s /dev/null %s/etc/systemd/system/systemd-udev-hwdb-update.service", fsdir),

		// set up directory for sshd_config (see below)
		fmt.Sprintf("mkdir -p %s/etc/ssh", fsdir),
	}

	for _, cmd := range cmds {
		var stderr, stdout string
		stdout, stderr, err = run(cmd)
		if err != nil {
			log.Printf("Command '%s' failed:\nstdout:: %s\nstderr: %s\nerr: %v", cmd, stdout, stderr, err)
			return
		}
	}

	sshd_config := `# Use most defaults for sshd configuration.
UsePrivilegeSeparation sandbox
Subsystem sftp internal-sftp
UseDNS no
`

	if err = ioutil.WriteFile(path.Join(fsdir, "/etc/ssh/sshd_config"), []byte(sshd_config), 0644); err != nil {
		log.Printf("Failed writing sshd_config: %v", err)
		return
	}

	// For expediency, generate the minimal viable SSH keys for the host, instead of the default set
	sshd_keygen := `[Unit]
	Description=Generate sshd host keys
	Before=sshd.service

	[Service]
	Type=oneshot
	RemainAfterExit=yes
	ExecStart=/usr/bin/ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N "" -b 768`
	if err = ioutil.WriteFile(path.Join(fsdir, "/etc/systemd/system/sshd-keygen.service"), []byte(sshd_keygen), 0644); err != nil {
		log.Printf("Failed writing sshd-keygen.service: %v", err)
		return
	}

	sshKeySrc := path.Join("fixtures", "id_rsa.pub")
	if err = nc.prepFleet(fsdir, nm.ip, sshKeySrc, fleetdBinPath, cfg); err != nil {
		log.Printf("Failed preparing fleetd in filesystem: %v", err)
		return
	}

	exec := strings.Join([]string{
		"/usr/bin/systemd-nspawn",
		"--bind-ro=/usr",
		"-b",
		fmt.Sprintf("-M %s%s", nc.name, name),
		"--capability=CAP_NET_BIND_SERVICE,CAP_SYS_TIME", // needed for ntpd
		"--network-bridge fleet0",
		fmt.Sprintf("-D %s", fsdir),
	}, " ")
	log.Printf("Creating nspawn container: %s", exec)
	err = nc.systemd(fmt.Sprintf("%s%s.service", nc.name, name), exec)
	if err != nil {
		log.Printf("Failed creating nspawn container: %v", err)
		return
	}

	nm.pid, err = nc.machinePID(name)
	if err != nil {
		log.Printf("Failed detecting machine %s%s PID: %v", nc.name, name, err)
		return
	}

	alarm := time.After(10 * time.Second)
	for {
		select {
		case <-alarm:
			log.Printf("Timed out waiting for machine to start")
			return
		default:
		}
		// TODO(jonboulle): probably a cleaner way to check here
		if _, _, e := nc.nsenter(nm.pid, "systemd-analyze"); e == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)

	}

	var stderr string
	cmd := fmt.Sprintf("ip addr add %s/16 dev host0", nm.ip)
	_, stderr, err = nc.nsenter(nm.pid, cmd)
	if err != nil {
		log.Printf("Failed adding IP address to container: %s", stderr)
		return
	}

	cmd = fmt.Sprintf("update-ssh-keys -u core -a fleet /opt/fleet/id_rsa.pub")
	_, _, err = nc.nsenter(nm.pid, cmd)
	if err != nil {
		log.Printf("Failed authorizing SSH key in container")
		return
	}

	_, _, err = nc.nsenter(nm.pid, "ln -s /opt/fleet/fleet.socket /etc/systemd/system/fleet.socket")
	if err != nil {
		log.Printf("Failed symlinking fleet.socket: %v", err)
		return
	}

	_, _, err = nc.nsenter(nm.pid, "ln -s /opt/fleet/fleet.service /etc/systemd/system/fleet.service")
	if err != nil {
		log.Printf("Failed symlinking fleet.service: %v", err)
		return
	}

	_, _, err = nc.nsenter(nm.pid, "systemctl start fleet.socket fleet.service")
	if err != nil {
		log.Printf("Failed starting fleet units: %v", err)
		return
	}

	return
}

func (nc *nspawnCluster) Destroy() error {
	for name := range nc.members {
		log.Printf("Destroying nspawn machine %s", name)
		nc.DestroyMember(name)
	}

	dir := path.Join(os.TempDir(), nc.name)
	if _, _, err := run(fmt.Sprintf("rm -fr %s", dir)); err != nil {
		log.Printf("Failed cleaning up cluster workspace: %v", err)
	}

	// TODO(bcwaldon): This returns 4 on success, but we can't easily
	// ignore just that return code. Ignore the returned error
	// altogether until this is fixed.
	run("etcdctl rm --recursive " + nc.keyspace())

	run("ip link del fleet0")

	return nil
}

func (nc *nspawnCluster) PoweroffMember(name string) (err error) {
	label := fmt.Sprintf("%s%s", nc.name, name)
	// The `machinectl poweroff` command does not cleanly shut down
	// the nspawn container, so we must use systemctl
	cmd := fmt.Sprintf("systemctl -M %s poweroff", label)
	_, _, err = run(cmd)
	if err != nil {
		log.Printf("Command '%s' failed: %v", cmd, err)
	}
	return
}

func (nc *nspawnCluster) DestroyMember(name string) error {
	dir := path.Join(os.TempDir(), nc.name, name)
	label := fmt.Sprintf("%s%s", nc.name, name)
	cmds := []string{
		fmt.Sprintf("machinectl terminate %s", label),
		fmt.Sprintf("rm -f /run/systemd/system/machine-%s.scope", label),
		fmt.Sprintf("rm -f /run/systemd/system/%s.service", label),
		fmt.Sprintf("rm -fr /run/systemd/system/%s.service.d", label),
		fmt.Sprintf("rm -r %s", dir),
	}

	for _, cmd := range cmds {
		_, _, err := run(cmd)
		if err != nil {
			log.Printf("Command '%s' failed, but operation will continue: %v", cmd, err)
		}
	}

	// Unfortunately nspawn doesn't always seem to tear down the interfaces
	// in time, which can result in subsequent tests failing
	run(fmt.Sprintf("ip link del vb-%s", label))

	if err := nc.systemdReload(); err != nil {
		log.Printf("Failed systemd daemon-reload: %v", err)
	}

	delete(nc.members, name)

	return nil
}

func (nc *nspawnCluster) systemdReload() error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}
	conn.Reload()
	return nil
}

func (nc *nspawnCluster) systemd(unitName, exec string) error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}

	props := []dbus.Property{
		dbus.PropExecStart(strings.Split(exec, " "), false),
	}

	log.Printf("Creating transient systemd unit %s", unitName)

	res1 := make(chan string)
	if _, err = conn.StartTransientUnit(unitName, "replace", props, res1); err != nil {
		log.Printf("Failed creating transient unit %s: %v", unitName, err)
		return err
	}
	<-res1

	res2 := make(chan string)
	_, err = conn.StartUnit(unitName, "replace", res2)
	if err != nil {
		log.Printf("Failed starting transient unit %s: %v", unitName, err)
		return err
	}

	<-res2
	return nil
}

// wait up to 10s for a machine to be started
func (nc *nspawnCluster) machinePID(name string) (int, error) {
	for i := 0; i < 100; i++ {
		mach := fmt.Sprintf("%s%s", nc.name, name)
		stdout, _, err := run(fmt.Sprintf("machinectl show -p Leader %s", mach))
		if err != nil {
			if i != -1 {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return -1, fmt.Errorf("failed detecting machine %s status: %v", mach, err)
		}

		out := strings.SplitN(strings.TrimSpace(stdout), "=", 2)
		return strconv.Atoi(out[1])
	}
	return -1, fmt.Errorf("unable to detect machine PID")
}

func (nc *nspawnCluster) nsenter(pid int, cmd string) (string, string, error) {
	cmd = fmt.Sprintf("nsenter -t %d -m -n -p -- %s", pid, cmd)
	return run(cmd)
}

func NewNspawnCluster(name string) (Cluster, error) {
	nc := &nspawnCluster{name, map[string]*nspawnMember{}}
	err := nc.prepCluster()
	return nc, err
}

func run(command string) (string, string, error) {
	log.Printf(command)
	var stdoutBytes, stderrBytes bytes.Buffer
	parts := strings.Split(command, " ")
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = &stdoutBytes
	cmd.Stderr = &stderrBytes
	err := cmd.Run()
	return stdoutBytes.String(), stderrBytes.String(), err
}

func copyFile(src, dst string, mode int) error {
	log.Printf("Copying %s -> %s", src, dst)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}

	if err = os.Chmod(dst, os.FileMode(mode)); err != nil {
		return err
	}

	return nil
}
