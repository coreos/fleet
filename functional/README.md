## fleet functional tests

This functional test suite deploys a fleet cluster nspawn containers and asserts fleet is functioning properly.
It shares etcd deployed on the host machine with each of the nspawn containers.

The caller must do two things before running the tests:

1. Ensure ssh-agent is running with the functional-testing SSH key. The `SSH_AUTH_SOCK` environment variable must be set.

```
$ ssh-agent
$ ssh-add fleet/functional/fixtures/id_rsa
$ echo $SSH_AUTH_SOCK
/tmp/ssh-kwmtTOsL7978/agent.7978
```
2. The `FLEETCTL_BIN` environment variable must point to the fleetctl binary that should be used to drive the actual tests.

```
$ export FLEETCTL_BIN=/usr/bin/fleetctl
```

Then the tests can be run (probably as root):

```
$ go test github.com/coreos/fleet/functional
ok  	github.com/coreos/fleet/functional	9.479s
```
