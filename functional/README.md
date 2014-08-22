## fleet functional tests

This functional test suite deploys a fleet cluster using nspawn containers, and asserts fleet is functioning properly.

It shares an instance of etcd deployed on the host machine with each of the nspawn containers.

It's recommended to run this in a virtual machine environment on CoreOS (e.g. using coreos-vagrant). The only dependency for the tests not provided on the CoreOS image is `go`.

The caller must do three things before running the tests:

1. Ensure an ssh-agent is running and the functional-testing identity is loaded. The `SSH_AUTH_SOCK` environment variable must be set.

```
$ ssh-agent
$ ssh-add fleet/functional/fixtures/id_rsa
$ echo $SSH_AUTH_SOCK
/tmp/ssh-kwmtTOsL7978/agent.7978
```
2. Ensure the `FLEETD_BIN` and `FLEETCTL_BIN` environment variables point to the respective fleetd and fleetctl binaries that should be used to drive the actual tests.

```
$ export FLEETD_BIN=/path/to/fleetd
$ export FLEETCTL_BIN=/path/to/fleetctl
```

3. Make sure etcd is running on the host system.

```
$ systemctl start etcd
```

Then the tests can be run with:

```
# go test github.com/coreos/fleet/functional
```

Since the tests utilize `systemd-nspawn`, this needs to be invoked as sudo/root.

An example test session using coreos-vagrant follows. This assumes that go is available in `/home/core/go` and the fleet repository in `/home/core/fleet` on the target machine (the easiest way to achieve this is to use shared folders).
```
vagrant ssh core-01 -- -A
export GOROOT="$(pwd)/go"
export PATH="${GOROOT}/bin:$PATH"
cd fleet
ssh-add functional/fixtures/id_rsa
export GOPATH="$(pwd)/gopath"
export FLEETD_BIN="$(pwd)/bin/fleet"
export FLEETCTL_BIN="$(pwd)/bin/fleetctl"
sudo -E env PATH=$PATH go test github.com/coreos/fleet/functional -v
```

If the tests are aborted partway through, it's currently possible for them to leave residual state as a result of the systemd-nspawn operations. This can be cleaned up using the `clean.sh` script.
