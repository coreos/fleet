# Metrics

**NOTE: The metrics feature is considered experimental. We may add/change/remove metrics without warning in future releases.**

fleet uses [Prometheus][prometheus] for metrics reporting in the server. The metrics can be used for real-time monitoring and debugging.

See the [etcd metrics.md][etcd-metrics] for more information about metrics used in etcd and Prometheus.

The naming of metrics follows the suggested best practice of Prometheus.  A metric name has an fleet prefix as its namespace and a subsystem prefix (for example engine and registry).

To get metrics data you have to request data from fleetd Unix domain socket:

```sh
curl --unix-socket /var/run/fleet.sock http:/metrics
```

If you've configured fleetd to [listen TCP socket][tcp-api], please use this command:

```sh
curl http://127.0.0.1:49153/metrics
```

fleetd exposes the following metrics:

| Name                                    | Description                                      | Type      |
|-----------------------------------------|--------------------------------------------------|-----------|
| engine_leader_start_time                | Timestamp when this fleetd became leader         | Gauge     |
| engine_task_count_total                 | The total number of executed tasks               | Counter   |
| engine_task_failure_count_total         | The total number of failed tasks                 | Counter   |
| engine_reconcile_count_total            | The total number of reconcile rounds             | Counter   |
| engine_reconcile_duration_second        | The latency distribution of reconcile rounds     | Histogram |
| engine_reconcile_failure_count_total    | The total number of reconcile failures           | Counter   |
| registry_operation_count_total          | The total number of registry operations          | Counter   |
| registry_operation_failed_count_total   | The total number of failed registry operations   | Counter   |
| registry_operation_duration_second      | The latency distribution of registry operations  | Histogram |

[etcd-metrics]: https://github.com/coreos/etcd/blob/master/Documentation/metrics.md
[prometheus]: http://prometheus.io/
[tcp-api]: deployment-and-configuration.md#api
