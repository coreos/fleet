# Metrics

**NOTE: The metrics feature is considered experimental. We may add/change/remove metrics without warning in future releases.**

fleet uses [Prometheus](http://prometheus.io/) for metrics reporting in the
server. The metrics can be used for real-time monitoring and debugging.

See the [etcd
metrics.md](https://github.com/coreos/etcd/blob/master/Documentation/metrics.md)
for more information about metrics used in etcd and Prometheus.

The naming of metrics follows the suggested best practice of Prometheus.
A metric name has an fleet prefix as its namespace and a subsystem prefix (for
example engine and registry).

fleet now exposes the following metrics:

## fleetd

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
