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

`missing_labels` is a comma-separated list of the configured labels absent from
that namespace. A label with an empty value is considered present; only label
presence is checked. With no required labels configured, the collector emits a
zero count, no detail samples, and a zero creation counter. The creation counter
is incremented when the informer receives an Add event for a non-whitelisted
namespace missing one or more labels; updates and deletes do not change it.
The initial informer list is also represented as Add events, so already-existing
dangerous namespaces are not missed when monitoring starts. The counter is
process-local and resets after a process restart, as with other Prometheus
counters.

To detect a newly observed dangerous namespace even when it is deleted before a
scrape, use a range query such as:

```promql
increase(sealos_namespace_missing_labels_created_total[5m]) > 0
```
