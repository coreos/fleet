# Remote fleet Access

fleet does not yet have any custom authentication, so security of a given fleet cluster depends on a user's ability to access any host in that cluster. The suggested method of authentication is public SSH keys and ssh-agents. The `fleetctl` command-line tool can assist in this by natively interacting with an ssh-agent to authenticate itself.

This requires two things:

1. SSH access for a user to at least one host in the cluster
2. ssh-agent running on a user's machine with the necessary identity

Authorizing a user's SSH key within a cluster is up to the deployer. See the [deployment doc][d] for help doing this.

[d]: https://github.com/coreos/fleet/blob/master/Documentation/deployment.md

Running an ssh-agent is the responsibility of the user. Many unix-based distros conveniently provide the necessary tools on a base install, or in an ssh-related package. For example, Ubuntu provides the `ssh-agent` and `ssh-add` binaries in the `openssh-client` package. If you cannot find the necessary binaries on your system, please consult your distro's documentation.

Assuming you have the tools installed, simply ensure ssh-agent has the necessary identity:

```
$ ssh-add ~/.ssh/id_rsa
Identity added: id_rsa (~/.ssh/id_rsa)
$ ssh-add -l
2048 31:c3:50:2b:44:f9:7f:28:6b:62:96:37:c7:c1:b5:c2 id_rsa (RSA)
```

To verify the ssh-agent and remote hosts are properly configured, simply connect directly to a host in the fleet cluster using `ssh`. Configure `fleetctl` to tunnel through that host by setting the `--tunnel` flag or exporting the `FLEETCTL_TUNNEL` environment variable:

```
$ fleetctl --tunnel 192.0.2.14:2222 list-units
...
$ FLEETCTL_TUNNEL=192.0.2.14:2222 fleetctl list-units
...
```

## Vagrant

Things get a bit more complicated when using [vagrant][v], as access to your hosts is abstracted away from the user. This makes it a bit more complicated to run `fleetctl` from your local laptop, but it's still relatively easy to configure.

[v]: http://www.vagrantup.com/

First, find the identity file used by vagrant to authenticate access to your hosts. The `vagrant` binary provides a convenient `ssh-config` command to help do this. Running `vagrant ssh-config` from a Vagrant project directory will produce something like this:

```
% vagrant ssh-config
Host default
  HostName 127.0.0.1
  User core
  Port 2222
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  IdentityFile /Users/bcwaldon/.vagrant.d/insecure_private_key
  IdentitiesOnly yes
  LogLevel FATAL
  ForwardAgent yes
```

The output communicates exactly how the connection to a vagrant host is made when calling `vagrant ssh`. Using the `HostName`, `Port` and `IdentityFile` options, we can bypass `vagrant ssh` and connect directly:

```
$ ssh -p 2222 -i /Users/bcwaldon/.vagrant.d/insecure_private_key core@127.0.0.1
Last login: Thu Feb 20 05:39:51 UTC 2014 from 10.0.2.2 on pts/1
   ______                ____  _____
  / ____/___  ________  / __ \/ ___/
 / /   / __ \/ ___/ _ \/ / / /\__ \
/ /___/ /_/ / /  /  __/ /_/ /___/ /
\____/\____/_/   \___/\____//____/
core@localhost ~ $
```

Now, let's get `fleetctl` working with these parameters:

```
$ vagrant ssh-config | sed -n "s/IdentityFile//gp" | xargs ssh-add
Identity added: /Users/bcwaldon/.vagrant.d/insecure_private_key (/Users/bcwaldon/.vagrant.d/insecure_private_key)
$ export FLEETCTL_TUNNEL="$(vagrant ssh-config | sed -n "s/[ ]*HostName[ ]*//gp"):$(vagrant ssh-config | sed -n "s/[ ]*Port[ ]*//gp")"
$ echo $FLEETCTL_TUNNEL
127.0.0.1:2222
$ fleetctl list-machines
...
```

The `ssh-add` command need only be run once for all Vagrant hosts. You will have to set `FLEETCTL_TUNNEL` specifically for each vagrant host with which you interact.
