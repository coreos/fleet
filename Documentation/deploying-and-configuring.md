# Deploying & Configuring

## Deploying Coreinit

Coreinit needs to be running on each machine that's part of the cluster. On each CoreOS machine, place the `coreinit` and `corectl` binaries in `/home/core`, the unit into `/media/state/units/coreinit.service`, and the config file into `/home/core/coreinit.conf`.

```
$ cat /media/state/units/coreinit.service
[Unit]
Description=coreinit

[Service]
ExecStart=/home/core/coreinit -config_file /home/core/coreinit.conf
```

To start your coreinit unit, run `sudo systemctl start coreinit.service`.

## Configuring Coreinit

You can configure coreinit with a config file or command line flags. See the [example config file][example-config] for more info.

[example-config]: https://github.com/coreos/coreinit/blob/master/coreinit.conf.sample

## Etcd Cluster

Coreinit assumes that you have an [etcd cluster][getting-started-etcd] running that it can talk to over `127.0.0.1:4001`.

[getting-started-etcd]: https://coreos.com/docs/cluster-management/setup/getting-started-with-etcd
