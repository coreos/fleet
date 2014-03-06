package platform

import (
	"bytes"
	"fmt"
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
	name  string
	count int
}

func (nc *nspawnCluster) Create(count int) error {
	rangeStart := nc.count
	rangeEnd := nc.count + count
	for i := rangeStart; i < rangeEnd; i++ {
		log.Printf("Creating nspawn machine %d in cluster %s", i, nc.name)
		nc.count += 1
		if err := nc.create(nc.name, i); err != nil {
			return err
		}
	}
	return nil
}

func (nc *nspawnCluster) prep() (err error) {
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

func (nc *nspawnCluster) prepFleet(dir, ip string, publicSSHKey []byte) error {
	cmd := fmt.Sprintf("mkdir -p %s/opt/fleet", dir)
	if _, _, err := run(cmd); err != nil {
		return err
	}

	cfgTmpl := `verbosity=2
etcd_servers=["http://172.17.0.1:4001"]	
public_ip=%s
`
	cfgContents := fmt.Sprintf(cfgTmpl, ip)
	cfgPath := path.Join(dir, "opt", "fleet", "fleet.conf")
	if err := ioutil.WriteFile(cfgPath, []byte(cfgContents), 0644); err != nil {
		return err
	}

	unitContents := `[Service]
ExecStart=/usr/bin/fleet -config /opt/fleet/fleet.conf
`
	unitPath := path.Join(dir, "opt", "fleet", "fleet-local.service")
	if err := ioutil.WriteFile(unitPath, []byte(unitContents), 0644); err != nil {
		return err
	}

	sshKeyPath := path.Join(dir, "opt", "fleet", "id_rsa.pub")
	if err := ioutil.WriteFile(sshKeyPath, publicSSHKey, 0644); err != nil {
		return err
	}

	return nil
}

func (nc *nspawnCluster) create(name string, num int) (err error) {
	basedir := path.Join(os.TempDir(), name)
	fsdir := path.Join(basedir, strconv.Itoa(num), "fs")
	cmds := []string{
		fmt.Sprintf("mkdir -p %s", fsdir),
		fmt.Sprintf("mount -o bind / %s", fsdir),
		fmt.Sprintf("mount -t tmpfs tmpfs %s", path.Join(fsdir, "home")),
		fmt.Sprintf("mount -t tmpfs tmpfs %s", path.Join(fsdir, "opt")),
		fmt.Sprintf("mount -t tmpfs tmpfs %s", path.Join(fsdir, "srv")),
		fmt.Sprintf("mount -t tmpfs tmpfs %s", path.Join(fsdir, "var")),
		fmt.Sprintf("mount -t tmpfs tmpfs %s", path.Join(fsdir, "etc/systemd/system")),
		fmt.Sprintf("ln -s /dev/null %s", path.Join(fsdir, "/etc/systemd/system/etcd.service")),
	}

	for _, cmd := range cmds {
		_, _, err = run(cmd)
		if err != nil {
			log.Printf("Command '%s' failed: %v", cmd, err)
			return
		}
	}

	publicSSHKey, err := ioutil.ReadFile(path.Join("fixtures", "id_rsa.pub"))
	if err != nil {
		log.Printf("Failed reading public SSH key: %v", err)
		return err
	}

	ip := fmt.Sprintf("172.17.1.%d", 100+num)
	if err = nc.prepFleet(fsdir, ip, publicSSHKey); err != nil {
		log.Printf("Failed preparing fleet in filesystem: %v", err)
		return
	}

	exec := fmt.Sprintf("/usr/bin/systemd-nspawn -b -M %s%d --network-bridge fleet0 -D %s", name, num, fsdir)
	err = nc.systemd(fmt.Sprintf("%s%d.service", name, num), exec)
	if err != nil {
		return
	}

	time.Sleep(time.Second)

	cmd := fmt.Sprintf("ip addr add %s/16 dev host0", ip)
	err = nc.nsenter(name, num, cmd)
	if err != nil {
		log.Printf("Failed adding IP address to container")
		return
	}

	cmd = fmt.Sprintf("update-ssh-keys -u core -a fleet /opt/fleet/id_rsa.pub")
	err = nc.nsenter(name, num, cmd)
	if err != nil {
		log.Printf("Failed authorizing SSH key in container")
		return
	}

	err = nc.nsenter(name, num, "ln -s /opt/fleet/fleet-local.service /run/systemd/system/fleet-local.service")
	if err != nil {
		log.Printf("Failed symlinking fleet-local.service: %v", err)
		return
	}

	err = nc.nsenter(name, num, "systemctl start fleet-local.service")
	if err != nil {
		log.Printf("Failed starting fleet-local.service: %v", err)
		return
	}

	return nil
}

func (nc *nspawnCluster) DestroyAll() error {
	for i := 0; i < nc.count; i++ {
		log.Printf("Destroying nspawn machine %d", i)
		nc.destroy(nc.name, i)
	}

	if err := nc.systemdReload(); err != nil {
		log.Printf("Failed systemd daemon-reload: %v", err)
	}

	dir := path.Join(os.TempDir(), nc.name)
	if _, _, err := run(fmt.Sprintf("rm -fr %s", dir)); err != nil {
		log.Printf("Failed cleaning up cluster workspace: %v", err)
	}

	// TODO(bcwaldon): This returns 4 on success, but we can't easily
	// ignore just that return code. Ignore the returned error
	// altogether until this is fixed.
	run("etcdctl rm --recursive /_coreos.com/fleet")

	return nil
}

func (nc *nspawnCluster) destroy(name string, num int) error {
	dir := path.Join(os.TempDir(), name, strconv.Itoa(num))
	cmds := []string{
		fmt.Sprintf("systemctl stop %s%d.service", name, num),
		fmt.Sprintf("rm -r /run/systemd/system/%s%d.service", name, num),
		fmt.Sprintf("umount --recursive %s/fs", dir),
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

func (nc *nspawnCluster) machinePID(name string, num int) (int, error) {
	mach := fmt.Sprintf("%s%d", name, num)
	stdout, _, err := run(fmt.Sprintf("machinectl status %s", mach))
	if err != nil {
		return -1, fmt.Errorf("Failed detecting machine %s status: %v", mach, err)
	}

	re := regexp.MustCompile("Leader:\\s(.*\\d)")
	pid := re.FindStringSubmatch(stdout)[1]
	if pid == "" {
		return -1, fmt.Errorf("Could not cast result '%s' to int", pid)
	}
	return strconv.Atoi(pid)
}

func (nc *nspawnCluster) nsenter(name string, num int, cmd string) error {
	pid, err := nc.machinePID(name, num)
	if err != nil {
		log.Printf("Failed detecting machine %s%d PID: %v", name, num, err)
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
	nc := &nspawnCluster{name, 0}
	err := nc.prep()
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
