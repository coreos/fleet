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

var fleetBinPath string

func init() {
	fleetBinPath = os.Getenv("FLEET_BIN")
	if fleetBinPath == "" {
		fmt.Println("FLEET_BIN environment variable must be set")
		os.Exit(1)
	} else if _, err := os.Stat(fleetBinPath); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

type member struct {
	ip  string
	pid int
}

type nspawnCluster struct {
	name    string
	members map[string]*member
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

func (nc *nspawnCluster) WaitForNActiveUnits(count int) (map[string]util.UnitState, error) {
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

func (nc *nspawnCluster) prepFleet(dir, ip, sshKeySrc, fleetBinSrc string, cfg MachineConfig) error {
	cmd := fmt.Sprintf("mkdir -p %s/opt/fleet", dir)
	if _, _, err := run(cmd); err != nil {
		return err
	}

	relSSHKeyDst := path.Join("opt", "fleet", "id_rsa.pub")
	sshKeyDst := path.Join(dir, relSSHKeyDst)
	if err := copyFile(sshKeySrc, sshKeyDst, 0644); err != nil {
		return err
	}

	fleetBinDst := path.Join(dir, "opt", "fleet", "fleet")
	if err := copyFile(fleetBinSrc, fleetBinDst, 0755); err != nil {
		return err
	}

	cfgTmpl := `verbosity=2
etcd_servers=["http://172.17.0.1:4001"]	
etcd_key_prefix=%s
public_ip=%s
verify_units=%s
authorized_keys_file=%s
`
	cfgContents := fmt.Sprintf(cfgTmpl, nc.keyspace(), ip, strconv.FormatBool(cfg.VerifyUnits), relSSHKeyDst)
	cfgPath := path.Join(dir, "opt", "fleet", "fleet.conf")
	if err := ioutil.WriteFile(cfgPath, []byte(cfgContents), 0644); err != nil {
		return err
	}

	unitContents := `[Service]
ExecStart=/opt/fleet/fleet -config /opt/fleet/fleet.conf
`
	unitPath := path.Join(dir, "opt", "fleet", "fleet.service")
	if err := ioutil.WriteFile(unitPath, []byte(unitContents), 0644); err != nil {
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

func (nc *nspawnCluster) CreateMember(name string, cfg MachineConfig) (err error) {
	log.Printf("Creating nspawn machine %s in cluster %s", name, nc.name)
	ip, err := nc.findUsableIP()
	if err != nil {
		return err
	}
	nc.members[name] = &member{ip: ip}

	basedir := path.Join(os.TempDir(), nc.name)
	fsdir := path.Join(basedir, name, "fs")
	cmds := []string{
		// set up directory for fleet service
		fmt.Sprintf("mkdir -p %s/etc/systemd/system", fsdir),

		// update-ca-certificates takes an inordinate amount of time, so simply mask it for now
		// until https://github.com/coreos/coreos-overlay/pull/681 is integrated
		fmt.Sprintf("ln -s /dev/null %s/etc/systemd/system/update-ca-certificates.service", fsdir),

		// since we're in a container we lack initrd bootstrapping magic
		// https://github.com/coreos/bootengine/blob/master/dracut/80setup-root/pre-pivot-setup-root.sh#L28
		// so until this is fixed, manually copy nsswitch.conf so that systemd-tmpfiles.service can access users it needs
		// https://github.com/coreos/init/pull/111/
		fmt.Sprintf("cp /etc/nsswitch.conf %s/etc", fsdir),

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

		// set up directory for machine-id (see below)
		fmt.Sprintf("mkdir -p %s/var/lib/dbus", fsdir),
	}

	for _, cmd := range cmds {
		var stderr, stdout string
		stdout, stderr, err = run(cmd)
		if err != nil {
			log.Printf("Command '%s' failed:\nstdout:: %s\nstderr: %s\nerr: %v", cmd, stdout, stderr, err)
			return
		}
	}

	// Write machine-id manually to override systemd picking up the host OS's machine-id in the case of the host being a KVM
	// (otherwise all smoke machines will have the same machine-id, causing havoc)
	// TODO(jonboulle): this should be fixed upstream in fdd25311706bd32580ec4d43211cdf4665d2f9de; remove once newer systemd is deployed
	uuid := fmt.Sprintf("0000000000000000000000000000000%s\n", name)
	if err = ioutil.WriteFile(path.Join(fsdir, "/var/lib/dbus/machine-id"), []byte(uuid), 0755); err != nil {
		log.Printf("Failed writing machine-id: %v", err)
		return
	}

	sshKeySrc := path.Join("fixtures", "id_rsa.pub")
	if err = nc.prepFleet(fsdir, ip, sshKeySrc, fleetBinPath, cfg); err != nil {
		log.Printf("Failed preparing fleet in filesystem: %v", err)
		return
	}

	exec := fmt.Sprintf("/usr/bin/systemd-nspawn --bind-ro=/usr -b -M %s%s --network-bridge fleet0 -D %s", nc.name, name, fsdir)
	log.Printf("Creating nspawn container: %s", exec)
	err = nc.systemd(fmt.Sprintf("%s%s.service", nc.name, name), exec)
	if err != nil {
		log.Printf("Failed creating nspawn container: %v", err)
		return
	}

	pid, err := nc.machinePID(name)
	if err != nil {
		log.Printf("Failed detecting machine %s%s PID: %v", nc.name, name, err)
		return err
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
		if _, _, e := nc.nsenter(pid, "systemd-analyze"); e == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)

	}

	nc.members[name].pid = pid

	var stderr string

	cmd := fmt.Sprintf("ip addr add %s/16 dev host0", ip)
	_, stderr, err = nc.nsenter(pid, cmd)
	if err != nil {
		log.Printf("Failed adding IP address to container: %s", stderr)
		return
	}

	cmd = fmt.Sprintf("update-ssh-keys -u core -a fleet /opt/fleet/id_rsa.pub")
	_, _, err = nc.nsenter(pid, cmd)
	if err != nil {
		log.Printf("Failed authorizing SSH key in container")
		return
	}

	_, _, err = nc.nsenter(pid, "ln -s /opt/fleet/fleet.service /etc/systemd/system/fleet.service")
	if err != nil {
		log.Printf("Failed symlinking fleet.service: %v", err)
		return
	}

	_, _, err = nc.nsenter(pid, "systemctl start fleet.service")
	if err != nil {
		log.Printf("Failed starting fleet.service: %v", err)
		return
	}

	return nil
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

	if _, err = conn.StartTransientUnit(unitName, "replace", props...); err != nil {
		log.Printf("Failed creating transient unit %s: %v", unitName, err)
		return err
	}

	_, err = conn.StartUnit(unitName, "replace")
	if err != nil {
		log.Printf("Failed starting transient unit %s: %v", unitName, err)
	}

	return err
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
	nc := &nspawnCluster{name, map[string]*member{}}
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
