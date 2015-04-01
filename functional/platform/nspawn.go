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
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/flt/Godeps/_workspace/src/github.com/coreos/go-systemd/dbus"

	"github.com/coreos/flt/functional/util"
)

const (
	fltAPIPort = 54728
)

var fltdBinPath string

func init() {
	fltdBinPath = os.Getenv("FLEETD_BIN")
	if fltdBinPath == "" {
		fmt.Println("FLEETD_BIN environment variable must be set")
		os.Exit(1)
	} else if _, err := os.Stat(fltdBinPath); err != nil {
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
	return string(m.id)
}

func (m *nspawnMember) IP() string {
	return m.ip
}

func (m *nspawnMember) Endpoint() string {
	return fmt.Sprintf("http://%s:%d", m.ip, fltAPIPort)
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
	return fmt.Sprintf("/flt_functional/%s", nc.name)
}

func (nc *nspawnCluster) Fltctl(m Member, args ...string) (string, string, error) {
	args = append([]string{"--experimental-api", "--endpoint=" + m.Endpoint()}, args...)
	return util.RunFltctl(args...)
}

func (nc *nspawnCluster) FltctlWithInput(m Member, input string, args ...string) (string, string, error) {
	args = append([]string{"--experimental-api", "--endpoint=" + m.Endpoint()}, args...)
	return util.RunFltctlWithInput(input, args...)
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
			stdout, _, err := nc.Fltctl(m, "list-units", "--no-legend", "--full", "--fields", "unit,active,machine")
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
			stdout, _, err := nc.Fltctl(m, "list-machines", "--no-legend", "--full", "--fields", "machine")
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

	if !strings.Contains(stdout, "flt0") {
		_, _, err = run("brctl addbr flt0")
		if err != nil {
			log.Printf("Failed adding flt0 bridge: %v", err)
			return
		}
	} else {
		log.Printf("Bridge flt0 already exists")
	}

	stdout, _, err = run("ip addr list flt0")
	if err != nil {
		log.Printf("Failed listing flt0 addresses: %v", err)
		return
	}

	if !strings.Contains(stdout, "172.17.0.1/16") {
		_, _, err = run("ip addr add 172.17.0.1/16 dev flt0")
		if err != nil {
			log.Printf("Failed adding 172.17.0.1/16 to flt0: %v", err)
			return
		}
	}

	_, _, err = run("ip link set flt0 up")
	if err != nil {
		log.Printf("Failed bringing up flt0 bridge: %v", err)
		return
	}

	return nil
}

func (nc *nspawnCluster) prepFlt(dir, ip, sshKeySrc, fltdBinSrc string) error {
	cmd := fmt.Sprintf("mkdir -p %s/opt/flt", dir)
	if _, _, err := run(cmd); err != nil {
		return err
	}

	relSSHKeyDst := path.Join("opt", "flt", "id_rsa.pub")
	sshKeyDst := path.Join(dir, relSSHKeyDst)
	if err := copyFile(sshKeySrc, sshKeyDst, 0644); err != nil {
		return err
	}

	fltdBinDst := path.Join(dir, "opt", "flt", "fltd")
	if err := copyFile(fltdBinSrc, fltdBinDst, 0755); err != nil {
		return err
	}

	cfgTmpl := `verbosity=2
etcd_servers=["http://172.17.0.1:4001"]	
etcd_key_prefix=%s
public_ip=%s
authorized_keys_file=%s
`
	cfgContents := fmt.Sprintf(cfgTmpl, nc.keyspace(), ip, relSSHKeyDst)
	cfgPath := path.Join(dir, "opt", "flt", "flt.conf")
	if err := ioutil.WriteFile(cfgPath, []byte(cfgContents), 0644); err != nil {
		return err
	}

	socketContents := fmt.Sprintf("[Socket]\nListenStream=%d\n", fltAPIPort)
	socketPath := path.Join(dir, "opt", "flt", "flt.socket")
	if err := ioutil.WriteFile(socketPath, []byte(socketContents), 0644); err != nil {
		return err
	}

	serviceContents := `[Service]
ExecStart=/opt/flt/fltd -config /opt/flt/flt.conf
`
	servicePath := path.Join(dir, "opt", "flt", "flt.service")
	if err := ioutil.WriteFile(servicePath, []byte(serviceContents), 0644); err != nil {
		return err
	}

	return nil
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

func (nc *nspawnCluster) createMember(id string) (m Member, err error) {
	nm := nspawnMember{
		id: id,
		ip: fmt.Sprintf("172.17.1.%s", id),
	}
	nc.members[id] = nm

	basedir := path.Join(os.TempDir(), nc.name)
	fsdir := path.Join(basedir, nm.ID(), "fs")
	cmds := []string{
		// set up directory for flt service
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
	if err = nc.prepFlt(fsdir, nm.IP(), sshKeySrc, fltdBinPath); err != nil {
		log.Printf("Failed preparing fltd in filesystem: %v", err)
		return
	}

	exec := strings.Join([]string{
		"/usr/bin/systemd-nspawn",
		"--bind-ro=/usr",
		"-b",
		fmt.Sprintf("-M %s%s", nc.name, nm.ID()),
		"--capability=CAP_NET_BIND_SERVICE,CAP_SYS_TIME", // needed for ntpd
		"--network-bridge flt0",
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
	cmd := fmt.Sprintf("ip addr add %s/16 dev host0", nm.IP())
	_, stderr, err = nc.nsenter(nm.pid, cmd)
	if err != nil {
		log.Printf("Failed adding IP address to container: %s", stderr)
		return
	}

	cmd = fmt.Sprintf("update-ssh-keys -u core -a flt /opt/flt/id_rsa.pub")
	_, _, err = nc.nsenter(nm.pid, cmd)
	if err != nil {
		log.Printf("Failed authorizing SSH key in container")
		return
	}

	_, _, err = nc.nsenter(nm.pid, "ln -s /opt/flt/flt.socket /etc/systemd/system/flt.socket")
	if err != nil {
		log.Printf("Failed symlinking flt.socket: %v", err)
		return
	}

	_, _, err = nc.nsenter(nm.pid, "ln -s /opt/flt/flt.service /etc/systemd/system/flt.service")
	if err != nil {
		log.Printf("Failed symlinking flt.service: %v", err)
		return
	}

	_, _, err = nc.nsenter(nm.pid, "systemctl start flt.socket flt.service")
	if err != nil {
		log.Printf("Failed starting flt units: %v", err)
		return
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

	run("ip link del flt0")

	return nil
}

func (nc *nspawnCluster) ReplaceMember(m Member) (Member, error) {
	count := len(nc.members)
	label := fmt.Sprintf("%s%s", nc.name, m.ID())

	// The `machinectl poweroff` command does not cleanly shut down
	// the nspawn container, so we must use systemctl
	cmd := fmt.Sprintf("systemctl -M %s poweroff", label)
	if _, stderr, _ := run(cmd); !strings.Contains(stderr, "Success") {
		return nil, fmt.Errorf("poweroff failed: %s", stderr)
	}

	var nm nspawnMember
	if m.ID() == "1" {
		nm = nc.members["2"]
	} else {
		nm = nc.members["1"]
	}
	mN := Member(&nm)

	if _, err := nc.WaitForNMachines(mN, count-1); err != nil {
		return nil, err
	}
	if err := nc.DestroyMember(m); err != nil {
		return nil, err
	}

	m, err := nc.createMember(m.ID())
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
