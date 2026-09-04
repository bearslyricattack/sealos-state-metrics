# Namespace Collector

The `namespace` collector watches Kubernetes Namespace objects and reports
namespaces that do not have all labels listed in `requiredLabels`.

Enable it with:

```yaml
enabledCollectors:
  - namespace

collectors:
  namespace:
    requiredLabels:
      - sealos.io/account
      - sealos.io/owner
    whitelist:
      - kube-system
```

The same setting can be provided through the environment variable
`COLLECTORS_NAMESPACE_REQUIRED_LABELS`, using a comma-separated list.
The optional `whitelist` setting (or
`COLLECTORS_NAMESPACE_WHITELIST`) excludes named namespaces from all metrics.

Assuming the global metrics namespace is `sealos`, the collector exports:

| Metric | Description | Labels |
| --- | --- | --- |
| `sealos_namespace_missing_labels_count` | Number of namespaces missing at least one required label | none |
| `sealos_namespace_missing_labels_info` | One sample for every namespace missing labels (value is always `1`) | `namespace`, `missing_labels` |
| `sealos_namespace_missing_labels_created_total` | Total namespace Add events that were missing at least one required label | none |
| `sealos_namespace_missing_labels_changed_total` | Total namespace Update events that changed one or more required labels | none |
| `sealos_namespace_missing_labels_created_by_namespace_total` | Total namespace Add events missing at least one required label, grouped by namespace | `namespace` |
| `sealos_namespace_missing_labels_updated_by_namespace_total` | Total namespace Update events that changed one or more required labels, grouped by namespace | `namespace` |

`missing_labels` is a comma-separated list of the configured labels absent from
that namespace. A label with an empty value is considered present; only label
presence is checked. With no required labels configured, the collector emits a
zero count, no detail samples, and zero event counters. The creation counter is
incremented when the informer receives an Add event for a non-whitelisted
namespace missing one or more labels; updates and deletes do not change it.
The initial informer list is also represented as Add events, so already-existing
dangerous namespaces are not missed when monitoring starts. The counter is
process-local and resets after a process restart, as with other Prometheus
counters.

The change counter is incremented when an Update event changes the presence or
value of at least one required label. Updates to other labels, metadata-only
updates, and deletes do not change it. Both adding and removing a required label
are counted as changes.

The `created_by_namespace_total` and `updated_by_namespace_total` counters retain
the namespace name as a label so an alert can identify the object that caused an
event. They are process-local counters and are retained after the Namespace is
deleted, but reset when the collector process restarts. A namespace that has
only update events still has a zero-valued creation series, and vice versa. Do
not add the current `missing_labels` value to these event metrics; use the
`missing_labels_info` metric for current state details.

To detect a newly observed dangerous namespace even when it is deleted before a
scrape, use a range query such as:

```promql
increase(sealos_namespace_missing_labels_created_total[5m]) > 0
```

For an alert that identifies the Namespace, use the per-namespace counter:

```promql
increase(sealos_namespace_missing_labels_created_by_namespace_total[5m]) > 0
or
increase(sealos_namespace_missing_labels_updated_by_namespace_total[5m]) > 0
```

The `namespace` label from the matching series is available to Grafana alert
annotations, for example `{{ $labels.namespace }}`.

Because a per-namespace event series is first created when that event is
observed, the first sample may not have a preceding zero baseline. If the
alert must catch a one-off event that occurs before the first scrape of that
series, use an event timestamp Gauge and compare it with `time()` instead of
relying only on `increase()`.
