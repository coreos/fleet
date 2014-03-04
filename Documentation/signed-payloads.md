# Signed Payloads

fleet supports payload authorization through public key cryptography.
A user may sign a payload with one or more public SSH keys when submitting it to the cluster.
Agents can then verify the provided signatures based on a set of preconfigured SSH keys.

NOTE: Signature creation and validation are disabled by default.

## Client

SSH keys are provided by a user through an ssh-agent.
Signatures are generated from all identities added to the agent.

To enable signature creation, simply pass the `--sign` flag to the `fleetctl submit` and `fleetctl start` commands.

## Server

A fleet server uses a pre-configured set of public SSH keys to validate signatures of units before allowing them to be loaded into systemd.
This allows a deployer to ensure that only clients identified by an authorized SSH identity are able to run a unit in a cluster.

To enable payload validation on a fleet server, simply set `verify_units=true` in the config.
fleet will validate payloads with the keys in `~/.ssh/authorized_keys` by default.
A deployer may provide an alternate set of SSH keys to use for validation using the `authorized_keys_file` option.

See [more information](configuration.md) on configuring `fleet`.
