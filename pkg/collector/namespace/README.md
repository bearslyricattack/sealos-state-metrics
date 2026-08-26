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
```

The same setting can be provided through the environment variable
`COLLECTORS_NAMESPACE_REQUIRED_LABELS`, using a comma-separated list.

Assuming the global metrics namespace is `sealos`, the collector exports:

| Metric | Description | Labels |
| --- | --- | --- |
| `sealos_namespace_missing_labels_count` | Number of namespaces missing at least one required label | none |
| `sealos_namespace_missing_labels_info` | One sample for every namespace missing labels (value is always `1`) | `namespace`, `missing_labels` |

`missing_labels` is a comma-separated list of the configured labels absent from
that namespace. A label with an empty value is considered present; only label
presence is checked. With no required labels configured, the collector emits a
zero count and no detail samples.
