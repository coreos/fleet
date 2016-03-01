# fleet vs kubernetes Comparison Table

Here is a short comparison table which will help you to choose which tool is more suitable for you:

|Feature|Description|fleet|kubernetes|
|------------|-----------|-----|----------|
| built-in load balancer | Kubernetes has built-in services abstraction. Fleet doesn't provide such feature but it can [deploy solution][service-discovery] built on top of containers and load-balancer software. | - | x |
| systemd units management | Kubernetes runs only containers abstraction. Fleet can schedule [several kinds][fleet-unit-types] of [systemd units][systemd-units]. | x | - |
| runs docker/rkt containers | Kubernetes uses [Docker API][docker-api] to run Docker containers and [systemd-nspawn][systemd-nspawn] to run rkt containers. Fleet uses [systemd units][systemd-units] to run containers. | x | x |
| DNS integration | Kubernetes already has possibility to run internal [DNS service][k8s-skydns] for containers. In fleet you have to build you own solution which will use [sidekicks][sidekick] and [service discovery][service-discovery]. | - | x |
| graphical user interface | Kubernetes already has a [dashboard][k8s-dashboard] provided as an addon. Fleet has external independent [dashboard][fleet-ui] project. | - | x |
| schedule jobs depending on system resources | Kubernetes uses [compute resources][compute-resources] abstraction and distributes load accross the cluster. Fleet just schedules systemd units. | - | x |
| ACL | Kubernetes has built-in [roles][k8s-roles] which provide access restrictions. Fleet doesn't have ACL and [by design][security] everyone who has access to the etcd cluster can manage fleet units. | - | x |
| by-design rolling updates | Kubernetes has built-in [rolling updates][k8s-rolling-update] functionality. With fleet you have to manually schedule new units and then destroy old ones. | - | x |
| hosts' labels / metadata | Both Kubernetes and Fleet can schedule units depending on hosts' [labels][k8s-node-label] / [metadata][metadata]. | x | x |

[docker-api]: https://docs.docker.com/engine/reference/api/docker_remote_api/
[systemd-nspawn]: https://www.freedesktop.org/software/systemd/man/systemd-nspawn.html
[systemd-units]: https://www.freedesktop.org/software/systemd/man/systemd.unit.html
[compute-resources]: https://github.com/kubernetes/kubernetes/blob/master/docs/user-guide/compute-resources.md
[fleet-ui]: https://github.com/purpleworks/fleet-ui
[fleet-unit-types]: unit-files-and-scheduling.md#unit-requirements
[k8s-dashboard]: https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dashboard
[k8s-node-label]: https://github.com/kubernetes/kubernetes/tree/master/docs/user-guide/node-selection
[k8s-roles]: https://github.com/kubernetes/kubernetes/blob/master/docs/design/security.md#roles
[k8s-rolling-update]: https://github.com/kubernetes/kubernetes/blob/master/docs/user-guide/kubectl/kubectl_rolling-update.md
[k8s-skydns]: https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dns
[metadata]: unit-files-and-scheduling.md#user-defined-requirements
[security]: architecture.md#security
[service-discovery]: examples/service-discovery.md
[sidekick]: https://github.com/coreos/docs/blob/master/fleet/launching-containers-fleet.md#run-a-simple-sidekick
