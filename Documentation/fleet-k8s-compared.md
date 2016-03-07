# fleet Compared to Kubernetes

This table briefly compares key features of `fleet` to those in Kubernetes:

|Feature|Description|fleet|kubernetes|
|------------|-----------|-----|----------|
| Load balancer | Kubernetes has built-in *services* abstraction. Fleet doesn't provide this feature directly, but it can [deploy a stacked solution][service-discovery] built atop app containers and external load-balancer software. | - | x |
| Systemd service management | Kubernetes runs only containers grouped in *pods*. Fleet can manage [several kinds][fleet-unit-types] of [systemd units][systemd-units]. | x | - |
| Runs rkt or Docker containers | Kubernetes uses the [Docker API][docker-api] to schedule Docker containers on cluster nodes, and uses the standard host-level [systemd-nspawn][systemd-nspawn] to execute rkt containers. Fleet uses [systemd units][systemd-units] to run containers or any other processes on cluster nodes. | x | x |
| DNS integration | Kubernetes includes a cluster-internal [DNS service][k8s-skydns] for container use. In fleet, such a solution would be constructed from [sidekicks][sidekick] app containers and [service discovery][service-discovery]. | - | x |
| Graphical user interface (GUI) | The Kubernetes [dashboard][k8s-dashboard] provides a minimal cluster mangement GUI, and commercial solutions like [Tectonic][tectonic] elaborate this graphical management. Fleet also has an external [dashboard][fleet-ui] project. | - | x |
| Schedule jobs depending on system resources | Kubernetes uses the [compute resources][compute-resources] abstraction to decide how to distribute load across the cluster. Fleet simply schedules systemd units. | - | x |
| Access Control and ACLs | Kubernetes has built-in [roles][k8s-roles] to provide access restrictions. Fleet doesn't have ACLs, and [by design everyone who has access to the etcd cluster can manage fleet units][security]. | - | x |
| Rolling updates | Kubernetes has built-in [rolling update][k8s-rolling-update] functionality to automate the deployment of new app versions on the cluster. With fleet, new units are scheduled and old ones destroyed manually. | - | x |
| Labels / metadata | Both Kubernetes and Fleet can schedule units depending on hosts' [labels][k8s-node-label] or basic [metadata][metadata]. | x | x |


[compute-resources]: https://github.com/kubernetes/kubernetes/blob/master/docs/user-guide/compute-resources.md
[docker-api]: https://docs.docker.com/engine/reference/api/docker_remote_api/
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
[systemd-nspawn]: https://www.freedesktop.org/software/systemd/man/systemd-nspawn.html
[systemd-units]: https://www.freedesktop.org/software/systemd/man/systemd.unit.html
[tectonic]: https://tectonic.com
