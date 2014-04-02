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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/fleet/third_party/github.com/coreos/go-systemd/dbus"
)

type nspawnCluster struct {
	name    string
	members []string
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
public_ip=%s
verify_units=%s
authorized_keys_file=%s
`
	cfgContents := fmt.Sprintf(cfgTmpl, ip, strconv.FormatBool(cfg.VerifyUnits), relSSHKeyDst)
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

func (nc *nspawnCluster) CreateMultiple(count int, cfg MachineConfig) error {
	for i := 0; i < count; i++ {
		name := strconv.Itoa(len(nc.members) + i)
		if err := nc.create(name, cfg); err != nil {
			return err
		}
	}
	return nil
}

func (nc *nspawnCluster) create(name string, cfg MachineConfig) (err error) {
	log.Printf("Creating nspawn machine %s in cluster %s", name, nc.name)
	nc.members = append(nc.members, name)

	basedir := path.Join(os.TempDir(), nc.name)
	fsdir := path.Join(basedir, name, "fs")
	cmds := []string{
		fmt.Sprintf("mkdir -p %s/etc/systemd/system", fsdir),
		fmt.Sprintf("cp /etc/os-release %s/etc", fsdir),
		fmt.Sprintf("mkdir -p %s/usr", fsdir),
		fmt.Sprintf("ln -s usr/lib64 %s/lib64", fsdir),
		fmt.Sprintf("ln -s lib64 %s/lib", fsdir),
		fmt.Sprintf("ln -s usr/bin %s/bin", fsdir),
		fmt.Sprintf("ln -s usr/sbin %s/sbin", fsdir),
		fmt.Sprintf("mkdir -p %s/home/core", fsdir),
		fmt.Sprintf("chown core:core %s/home/core", fsdir),
	}

	for _, cmd := range cmds {
		_, _, err = run(cmd)
		if err != nil {
			log.Printf("Command '%s' failed: %v", cmd, err)
			return
		}
	}

	ip := fmt.Sprintf("172.17.1.%d", 100+len(nc.members))
	sshKeySrc := path.Join("fixtures", "id_rsa.pub")
	fleetBinSrc := path.Join("../", "bin", "fleet")
	if err = nc.prepFleet(fsdir, ip, sshKeySrc, fleetBinSrc, cfg); err != nil {
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

	time.Sleep(time.Second)

	cmd := fmt.Sprintf("ip addr add %s/16 dev host0", ip)
	err = nc.nsenter(name, cmd)
	if err != nil {
		log.Printf("Failed adding IP address to container")
		return
	}

	cmd = fmt.Sprintf("update-ssh-keys -u core -a fleet /opt/fleet/id_rsa.pub")
	err = nc.nsenter(name, cmd)
	if err != nil {
		log.Printf("Failed authorizing SSH key in container")
		return
	}

	err = nc.nsenter(name, "ln -s /opt/fleet/fleet.service /etc/systemd/system/fleet.service")
	if err != nil {
		log.Printf("Failed symlinking fleet.service: %v", err)
		return
	}

	err = nc.nsenter(name, "systemctl start fleet.service")
	if err != nil {
		log.Printf("Failed starting fleet.service: %v", err)
		return
	}

	return nil
}

func (nc *nspawnCluster) DestroyAll() error {
	for _, name := range nc.members {
		log.Printf("Destroying nspawn machine %s", name)
		nc.destroy(name)
	}

	// All members were destroyed, reset to nil
	nc.members = []string{}

	dir := path.Join(os.TempDir(), nc.name)
	if _, _, err := run(fmt.Sprintf("rm -fr %s", dir)); err != nil {
		log.Printf("Failed cleaning up cluster workspace: %v", err)
	}

	if err := nc.systemdReload(); err != nil {
		log.Printf("Failed systemd daemon-reload: %v", err)
	}

	// TODO(bcwaldon): This returns 4 on success, but we can't easily
	// ignore just that return code. Ignore the returned error
	// altogether until this is fixed.
	run("etcdctl rm --recursive /_coreos.com/fleet")

	return nil
}

func (nc *nspawnCluster) destroy(name string) error {
	dir := path.Join(os.TempDir(), nc.name, name)
	cmds := []string{
		fmt.Sprintf("machinectl terminate %s%s", nc.name, name),
		fmt.Sprintf("rm -f /run/systemd/system/%s%s.service", nc.name, name),
		fmt.Sprintf("rm -fr /run/systemd/system/%s%s.service.d", nc.name, name),
		fmt.Sprintf("rm -r %s", dir),
	}

	for _, cmd := range cmds {
		_, _, err := run(cmd)
		if err != nil {
			log.Printf("Command '%s' failed, but operation will continue: %v", cmd, err)
		}
	}

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

func (nc *nspawnCluster) machinePID(name string) (int, error) {
	for i := 0; i < 5; i++ {
		mach := fmt.Sprintf("%s%s", nc.name, name)
		stdout, _, err := run(fmt.Sprintf("machinectl status %s", mach))
		if err != nil {
			if i != 4 {
				time.Sleep(time.Second)
				continue
			}
			return -1, fmt.Errorf("Failed detecting machine %s status: %v", mach, err)
		}

		re := regexp.MustCompile("Leader:\\s(.*\\d)")
		pid := re.FindStringSubmatch(stdout)[1]
		if pid == "" {
			return -1, fmt.Errorf("Could not cast result '%s' to int", pid)
		}
		return strconv.Atoi(pid)
	}
	return -1, fmt.Errorf("Unable to detect machine PID")
}

func (nc *nspawnCluster) nsenter(name string, cmd string) error {
	pid, err := nc.machinePID(name)
	if err != nil {
		log.Printf("Failed detecting machine %s%s PID: %v", nc.name, name, err)
		return err
	}

	cmd = fmt.Sprintf("nsenter -t %d -m -n -p -- %s", pid, cmd)
	_, _, err = run(cmd)
	if err != nil {
		log.Printf("Command '%s' failed: %v", cmd, err)
		return err
	}

	return nil
}

func NewNspawnCluster(name string) (Cluster, error) {
	nc := &nspawnCluster{name, []string{}}
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
