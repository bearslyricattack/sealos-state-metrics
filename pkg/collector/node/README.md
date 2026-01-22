# Node Collector

The Node collector monitors Kubernetes node conditions and resource status.

## Configuration

### YAML Configuration

```yaml
collectors:
  node: {}
```

The Node collector has no configurable parameters. It automatically monitors all nodes in the cluster.

### Environment Variables

No environment variables are required for this collector.

## Metrics

### `sealos_node_condition`

**Type:** Gauge
**Labels:**
- `node`: Node name
- `condition`: Condition type (e.g., `Ready`, `MemoryPressure`, `DiskPressure`, `PIDPressure`, `NetworkUnavailable`)
- `status`: Condition status (`True`, `False`, `Unknown`)

**Values:**
- `1`: Condition is present
- `0`: Condition is not present (metric not emitted)

**Example:**
```promql
sealos_node_condition{node="worker-1",condition="Ready",status="True"} 1
sealos_node_condition{node="worker-1",condition="MemoryPressure",status="False"} 1
sealos_node_condition{node="worker-1",condition="DiskPressure",status="False"} 1
sealos_node_condition{node="worker-2",condition="Ready",status="Unknown"} 1
```

### Common Node Conditions

| Condition | Description |
|-----------|-------------|
| `Ready` | Node is healthy and ready to accept pods |
| `MemoryPressure` | Node is under memory pressure |
| `DiskPressure` | Node is under disk pressure |
| `PIDPressure` | Node is running too many processes |
| `NetworkUnavailable` | Node network is not correctly configured |

## Use Cases

### Alerting on Node Issues

```promql
# Alert when node is not Ready
sealos_node_condition{condition="Ready",status!="True"} == 1

# Alert on memory pressure
sealos_node_condition{condition="MemoryPressure",status="True"} == 1

# Alert on disk pressure
sealos_node_condition{condition="DiskPressure",status="True"} == 1
```

## Collector Type

**Type:** Informer
**Leader Election Required:** No

The Node collector uses Kubernetes informers to watch node condition changes in real-time.
