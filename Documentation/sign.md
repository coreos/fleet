# Signature

Sign module provides signing functions to secure data. It could generate signature for data, and verify whether or not the data fits the signature.

# Cover range

It is currently used for job payload only.

# Usage

The module is off on default now.

When turned on in fleet using flag `--verify_units`, it would check data in its coverage and block failed one.

For fleetctl, the commands related to unit files have `-sign` or `-verify` flag. `sign` flag enables the creation of unit file signature on uploading, while `verify` flag enforces the verification of unit files that it fetches.

# Implementation

fleet does not yet have any custom authentication, so security of a given fleet cluster depends on a user's ability to access any host in that cluster. The suggested method of authentication is public SSH keys and ssh-agents. See the [Remote fleet access][r] for help doing this.

[r]: remote-access.md

Sign module uses the suggested way of authentication to sign and verify data. To get the signature of data, it would connect to ssh-agent, and send sign request based on the instruction in [PROTOCOL.agent][p] section 2.6.2. On the other side, to check the integrity of data, it grabs public keys from ssh-agent or authrorized-key file in the local machine, which is at `~/.ssh/authorized_keys` by default, and verifies the correctness of the signature.

[p]: http://www.openbsd.org/cgi-bin/cvsweb/src/usr.bin/ssh/PROTOCOL.agent

# Storing on etcd

Signature could be stored on etcd through registry module. It creates the directory `/signing`, and stores the signature with $tag in `/signing/$tag`.
