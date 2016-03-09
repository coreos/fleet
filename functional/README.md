## fleet functional tests

This functional test suite deploys a fleet cluster using nspawn containers, and asserts fleet is functioning properly.

It shares an instance of etcd deployed on the host machine with each of the nspawn containers which use `172.18.0.1/16` network, so please make sure this network does not intersect with others.

It's recommended to run this in a virtual machine environment on CoreOS (e.g. using [Vagrant][test-in-vagrant]).

Since the tests utilize [`systemd-nspawn`][systemd-nspawn], this needs to be invoked as sudo/root.

If the tests are aborted partway through, it's currently possible for them to leave residual state as a result of the `systemd-nspawn` operations. This can be cleaned up using the `clean.sh` script.

### Using go flags in functional tests

`fleet/functional` scripts forwards all arguments directly to the `go` binary. This allows you to pass `-run regexp` argument to the `./test` script and run specific test functions. The example below shows how to run only `TestSchedule*` test functions:

```sh
$ sudo fleet/functional/test -run TestSchedule*
```

The test functions list could be printed using `grep` command:

```sh
$ cd fleet/functional
$ grep -r 'func Test' .
```

You can find a detailed description of the available test flags in the [Go documentation][golang-test-flags].

### Run tests in Vagrant

The recommended way to run the tests is to use the provided Vagrantfile, which will set up a single CoreOS instance with a one-member etcd cluster (configuration is applied using `user-data` [Cloud-Config][cloud-config] file located in this directory).
To do so, simply run the following commands on a system with Vagrant installed (see [Vagrant configuration][configure-vagrant] section of this doc)

```sh
$ git clone https://github.com/coreos/fleet
$ cd fleet/functional
$ ./run-in-vagrant
```

Vagrant's provision step includes go binaries download using `functional/provision/install_go.sh` script.

### Run tests in QEMU

If you don't want to use Vagrant or VirtualBox, you can run tests inside official CoreOS QEMU image. You have to make sure QEMU is installed on your system.

```sh
$ git clone https://github.com/coreos/fleet
$ cd fleet/functional
$ ./run-in-qemu
```

If you get `Could not access KVM kernel module: Permission denied` error message, please make sure your CPU supports hardware-assisted virtualization and try to run the script using `sudo`.

### Run tests inside other CoreOS platforms (BareMetal/EC2/GCE/libvirt/etc)

It's also possible to run the tests on CoreOS on other platforms. The following commands should be run *inside* the CoreOS instance.

```sh
$ git clone https://github.com/coreos/fleet
```

If you didn't configure etcd2 daemon yet, just run this script:

```sh
$ sudo fleet/functional/start_etcd
```

It will configure and start a one-member etcd cluster.

Then run the functional tests (script will download and unpack golang into home directory):

```sh
$ sudo fleet/functional/test
```

When `fleet/functional/test` can not find go binaries, it will download them automatically using `functional/provision/install_go.sh` script.

## Configure host environment to run Vagrant

### Debian/Ubuntu

#### Install Vagrant

```sh
sudo apt-get install -y git nfs-kernel-server
wget https://releases.hashicorp.com/vagrant/1.8.1/vagrant_1.8.1_x86_64.deb
sudo dpkg -i vagrant_1.8.1_x86_64.deb
```

#### Install VirtualBox

```sh
echo "deb http://download.virtualbox.org/virtualbox/debian $(lsb_release -sc) contrib" | sudo tee /etc/apt/sources.list.d/virtualbox.list
wget -q https://www.virtualbox.org/download/oracle_vbox.asc -O- | sudo apt-key add -
sudo apt-get update
sudo apt-get install -y build-essential linux-headers-`uname -r` dkms
sudo apt-get install -y VirtualBox-5.0
#Previous VirtualBox (if you have problems with nested virtualization, more info here: https://www.virtualbox.org/ticket/14965)
#sudo apt-get install -y VirtualBox-4.3
```

### CentOS/Fedora

**NOTE**: NFS and Vagrant doesn't work out of the box on CentOS 6.x, so it is recommended to use CentOS 7.x

#### Install Vagrant

```sh
sudo yum install -y git nfs-utils
sudo service nfs start
sudo yum install -y https://releases.hashicorp.com/vagrant/1.8.1/vagrant_1.8.1_x86_64.rpm
```

#### Install VirtualBox

```sh
source /etc/os-release
for id in $ID_LIKE $ID; do break; done
OS_ID=${id:-rhel}
curl http://download.virtualbox.org/virtualbox/rpm/$OS_ID/virtualbox.repo | sudo tee /etc/yum.repos.d/virtualbox.repo
sudo yum install -y make automake gcc gcc-c++ kernel-devel-`uname -r` dkms
sudo yum install -y VirtualBox-5.0
#Previous VirtualBox (if you have problems with nested virtualization, more info here: https://www.virtualbox.org/ticket/14965)
#sudo yum install -y VirtualBox-4.3
```

[cloud-config]: https://github.com/coreos/coreos-cloudinit/blob/master/Documentation/cloud-config.md
[configure-vagrant]: #configure-host-environment-to-run-vagrant
[golang-test-flags]: https://golang.org/cmd/go/#hdr-Description_of_testing_flags
[systemd-nspawn]: https://www.freedesktop.org/software/systemd/man/systemd-nspawn.html
[test-in-vagrant]: #run-tests-in-vagrant
