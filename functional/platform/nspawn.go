// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package platform

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/fleet/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
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
	uuid string
	id   string
	ip   string
	pid  int
}

func (m *nspawnMember) ID() string {
	return m.uuid
}

func (m *nspawnMember) IP() string {
	return m.ip
}

func (m *nspawnMember) Endpoint() string {
	return fmt.Sprintf("http://%s:%d", m.ip, fleetAPIPort)
}

type nspawnCluster struct {
	name    string
	maxID   int
	members map[string]nspawnMember
}

func (nc *nspawnCluster) nextID() string {
	nc.maxID++
	return strconv.Itoa(nc.maxID)
}

func (nc *nspawnCluster) keyspace() string {
	// TODO(jonboulle): generate this dynamically with atomic in order keys?
	return fmt.Sprintf("/fleet_functional/%s", nc.name)
}

func (nc *nspawnCluster) Fleetctl(m Member, args ...string) (string, string, error) {
	args = append([]string{"--endpoint=" + m.Endpoint()}, args...)
	return util.RunFleetctl(args...)
}

func (nc *nspawnCluster) FleetctlWithInput(m Member, input string, args ...string) (string, string, error) {
	args = append([]string{"--endpoint=" + m.Endpoint()}, args...)
	return util.RunFleetctlWithInput(input, args...)
}

func (nc *nspawnCluster) WaitForNActiveUnits(m Member, count int) (map[string][]util.UnitState, error) {
	var nactive int
	states := make(map[string][]util.UnitState)

	timeout := 15 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(250 * time.Millisecond)
loop:
	for {
		select {
		case <-alarm:
			return nil, fmt.Errorf("failed to find %d active units within %v (last found: %d)", count, timeout, nactive)
		case <-ticker:
			stdout, _, err := nc.Fleetctl(m, "list-units", "--no-legend", "--full", "--fields", "unit,active,machine")
			stdout = strings.TrimSpace(stdout)
			if err != nil {
				continue
			}

			lines := strings.Split(stdout, "\n")
			allStates := util.ParseUnitStates(lines)
			active := util.FilterActiveUnits(allStates)
			nactive = len(active)
			if nactive != count {
				continue
			}

			for _, state := range active {
				name := state.Name
				if _, ok := states[name]; !ok {
					states[name] = []util.UnitState{}
				}
				states[name] = append(states[name], state)
			}
			break loop
		}
	}

	return states, nil
}

func (nc *nspawnCluster) WaitForNMachines(m Member, count int) ([]string, error) {
	var machines []string
	timeout := 10 * time.Second
	alarm := time.After(timeout)

	ticker := time.Tick(250 * time.Millisecond)
loop:
	for {
		select {
		case <-alarm:
			return machines, fmt.Errorf("failed to find %d machines within %v", count, timeout)
		case <-ticker:
			stdout, _, err := nc.Fleetctl(m, "list-machines", "--no-legend", "--full", "--fields", "machine")
			if err != nil {
				continue
			}

			stdout = strings.TrimSpace(stdout)

			found := 0
			if stdout != "" {
				machines = strings.Split(stdout, "\n")
				found = len(machines)
			}

			if found != count {
				continue
			}

			break loop
		}
	}

	return machines, nil
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

	if !strings.Contains(stdout, "172.18.0.1/16") {
		_, _, err = run("ip addr add 172.18.0.1/16 dev fleet0")
		if err != nil {
			log.Printf("Failed adding 172.18.0.1/16 to fleet0: %v", err)
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

func (nc *nspawnCluster) insertFleetd(dir string) error {
	cmd := fmt.Sprintf("mkdir -p %s/opt/fleet", dir)
	if _, _, err := run(cmd); err != nil {
		return err
	}

	fleetdBinDst := path.Join(dir, "opt", "fleet", "fleetd")
	return copyFile(fleetdBinPath, fleetdBinDst, 0755)
}

func (nc *nspawnCluster) buildConfigDrive(dir, ip string) error {
	latest := path.Join(dir, "media/configdrive/openstack/latest")
	userPath := path.Join(latest, "user_data")
	if err := os.MkdirAll(latest, 0755); err != nil {
		return err
	}

	userFile, err := os.OpenFile(userPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer userFile.Close()

	etcd := "http://172.18.0.1:4001"
	return util.BuildCloudConfig(userFile, ip, etcd, nc.keyspace())
}

func (nc *nspawnCluster) Members() []Member {
	ms := make([]Member, 0)
	for _, nm := range nc.members {
		nm := nm
		ms = append(ms, Member(&nm))
	}
	return ms
}

func (nc *nspawnCluster) MemberCommand(m Member, args ...string) (string, error) {
	baseArgs := []string{"-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no", fmt.Sprintf("core@%s", m.IP())}
	args = append(baseArgs, args...)
	log.Printf("ssh %s", strings.Join(args, " "))
	var stdoutBytes bytes.Buffer
	cmd := exec.Command("ssh", args...)
	cmd.Stdout = &stdoutBytes
	err := cmd.Run()
	return stdoutBytes.String(), err
}

func (nc *nspawnCluster) CreateMember() (m Member, err error) {
	id := nc.nextID()
	log.Printf("Creating nspawn machine %s in cluster %s", id, nc.name)
	return nc.createMember(id)
}

func newMachineID() string {
	// drop the standard separators to match systemd
	return strings.Replace(uuid.New(), "-", "", -1)
}

func (nc *nspawnCluster) createMember(id string) (m Member, err error) {
	nm := nspawnMember{
		uuid: newMachineID(),
		id:   id,
		ip:   fmt.Sprintf("172.18.1.%s", id),
	}
	nc.members[nm.ID()] = nm

	basedir := path.Join(os.TempDir(), nc.name)
	fsdir := path.Join(basedir, nm.ID(), "fs")
	cmds := []string{
		// set up directory for fleet service
		fmt.Sprintf("mkdir -p %s/etc/systemd/system", fsdir),

		// minimum requirements for running systemd/coreos in a container
		fmt.Sprintf("mkdir -p %s/usr", fsdir),
		fmt.Sprintf("cp /etc/os-release %s/etc", fsdir),
		fmt.Sprintf("echo 'core:x:500:500:CoreOS Admin:/home/core:/bin/bash' > %s/etc/passwd", fsdir),
		fmt.Sprintf("echo 'core:x:500:' > %s/etc/group", fsdir),
		fmt.Sprintf("ln -s /proc/self/mounts %s/etc/mtab", fsdir),
		fmt.Sprintf("ln -s usr/lib64 %s/lib64", fsdir),
		fmt.Sprintf("ln -s lib64 %s/lib", fsdir),
		fmt.Sprintf("ln -s usr/bin %s/bin", fsdir),
		fmt.Sprintf("ln -s usr/sbin %s/sbin", fsdir),
		fmt.Sprintf("mkdir -p %s/home/core/.ssh", fsdir),
		fmt.Sprintf("chown -R 500:500 %s/home/core", fsdir),

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
	ExecStart=/usr/bin/ssh-keygen -t rsa -f /etc/ssh/ssh_host_rsa_key -N "" -b 1024`
	if err = ioutil.WriteFile(path.Join(fsdir, "/etc/systemd/system/sshd-keygen.service"), []byte(sshd_keygen), 0644); err != nil {
		log.Printf("Failed writing sshd-keygen.service: %v", err)
		return
	}

	if err = nc.insertFleetd(fsdir); err != nil {
		log.Printf("Failed preparing fleetd in filesystem: %v", err)
		return
	}

	if err = nc.buildConfigDrive(fsdir, nm.IP()); err != nil {
		log.Printf("Failed building config drive: %v", err)
		return
	}

	exec := strings.Join([]string{
		"/usr/bin/systemd-nspawn",
		"--bind-ro=/usr",
		"-b",
		"--uuid=" + nm.uuid,
		fmt.Sprintf("-M %s%s", nc.name, nm.ID()),
		"--capability=CAP_NET_BIND_SERVICE,CAP_SYS_TIME", // needed for ntpd
		"--network-bridge fleet0",
		fmt.Sprintf("-D %s", fsdir),
	}, " ")
	log.Printf("Creating nspawn container: %s", exec)
	err = nc.systemd(fmt.Sprintf("%s%s.service", nc.name, nm.ID()), exec)
	if err != nil {
		log.Printf("Failed creating nspawn container: %v", err)
		return
	}

	nm.pid, err = nc.machinePID(nm.ID())
	if err != nil {
		log.Printf("Failed detecting machine %s%s PID: %v", nc.name, nm.ID(), err)
		return
	}

	alarm := time.After(10 * time.Second)
	addr := fmt.Sprintf("%s:%d", nm.IP(), fleetAPIPort)
	for {
		select {
		case <-alarm:
			err = fmt.Errorf("Timed out waiting for machine to start")
			log.Printf("Starting %s%s failed: %v", nc.name, nm.ID(), err)
			return
		default:
		}
		log.Printf("Dialing machine: %s", addr)
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return Member(&nm), nil
}

func (nc *nspawnCluster) Destroy() error {
	for _, m := range nc.Members() {
		log.Printf("Destroying nspawn machine %s", m.ID())
		nc.DestroyMember(m)
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

func (nc *nspawnCluster) ReplaceMember(m Member) (Member, error) {
	count := len(nc.members)
	label := fmt.Sprintf("%s%s", nc.name, m.ID())

	// The `machinectl poweroff` command does not cleanly shut down
	// the nspawn container, so we must use systemctl
	cmd := fmt.Sprintf("systemctl -M %s poweroff", label)
	if _, stderr, _ := run(cmd); !strings.Contains(stderr, "Success") {
		if strings.Contains(stderr, "Warning! D-Bus connection terminated.") {
			log.Printf("poweroff failed: %s", stderr)
		} else {
			return nil, fmt.Errorf("poweroff failed: %s", stderr)
		}
	}

	var mN Member
	for id, nm := range nc.members {
		if id != m.ID() {
			mN = Member(&nm)
			break
		}
	}

	if _, err := nc.WaitForNMachines(mN, count-1); err != nil {
		return nil, err
	}
	if err := nc.DestroyMember(m); err != nil {
		return nil, err
	}

	m, err := nc.createMember(m.(*nspawnMember).id)
	if err != nil {
		return nil, err
	}

	if _, err := nc.WaitForNMachines(mN, count); err != nil {
		return nil, err
	}

	return m, nil
}

func (nc *nspawnCluster) DestroyMember(m Member) error {
	dir := path.Join(os.TempDir(), nc.name, m.ID())
	label := fmt.Sprintf("%s%s", nc.name, m.ID())
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

	delete(nc.members, m.ID())

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
	nc := &nspawnCluster{name: name, members: map[string]nspawnMember{}}
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
