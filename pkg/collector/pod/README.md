# Pod Collector

The Pod collector monitors container restart counts and identifies frequently restarting containers.

## Configuration

### YAML Configuration

```yaml
collectors:
  pod:
    namespaces: []
    restartThreshold: 5
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `namespaces` | []string | `[]` | List of namespaces to monitor (empty = all namespaces) |
| `restartThreshold` | int | `5` | Minimum restart count to report (only containers with restarts >= threshold are reported) |

### Environment Variables

| Environment Variable | Maps To | Example |
|---------------------|---------|---------|
| `COLLECTORS_POD_NAMESPACES` | `namespaces` | `default,kube-system` |
| `COLLECTORS_POD_RESTART_THRESHOLD` | `restartThreshold` | `10` |

## Metrics

### `sealos_pod_container_restarts_total`

**Type:** Gauge
**Labels:**
- `namespace`: Pod namespace
- `pod`: Pod name
- `container`: Container name
- `reason`: Last termination reason (e.g., `Error`, `OOMKilled`, `CrashLoopBackOff`)

**Description:** Total number of container restarts. Only containers with restart count >= `restartThreshold` are reported.

**Example:**
```promql
sealos_pod_container_restarts_total{namespace="default",pod="nginx-7d6f8c",container="nginx",reason="Error"} 12
sealos_pod_container_restarts_total{namespace="kube-system",pod="coredns-abc",container="coredns",reason="OOMKilled"} 8
```

## Use Cases

### Alerting on Restart Issues

```promql
# Alert when container has restarted more than 10 times
sealos_pod_container_restarts_total > 10

# Alert on OOMKilled containers
sealos_pod_container_restarts_total{reason="OOMKilled"} > 0

# Alert on crash loop containers
sealos_pod_container_restarts_total{reason=~".*Crash.*"} > 5
```

### Monitoring Restart Rate

```promql
# Rate of restarts over 5 minutes
rate(sealos_pod_container_restarts_total[5m])

# Containers with increasing restart rate
increase(sealos_pod_container_restarts_total[10m]) > 3
```

## Collector Type

**Type:** Informer
**Leader Election Required:** No

The Pod collector uses Kubernetes informers to watch pod status changes in real-time.
