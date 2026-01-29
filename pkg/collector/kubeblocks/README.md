# KubeBlocks Collector

The KubeBlocks collector monitors KubeBlocks Cluster resources and exposes their status as Prometheus metrics.

## Features

- **Dynamic CRD monitoring**: Uses dynamic client to watch KubeBlocks Cluster CRDs
- **Multi-namespace support**: Can monitor clusters in specific namespaces or all namespaces
- **Cluster phase tracking**: Monitors cluster lifecycle phases (Creating, Running, Failed, etc.)
- **Component status**: Tracks individual component phases and pod readiness
- **Condition monitoring**: Exposes cluster conditions for detailed health tracking
- **Efficient updates**: Uses Kubernetes informers for real-time updates with minimal overhead

## Configuration

### YAML Configuration

```yaml
collectors:
  kubeblocks:
    enabled: true
    namespaces:
      - ns-user1
      - ns-user2
    resyncPeriod: "10m"
    includePhaseMetric: true
    includeComponentMetrics: true
    includeConditionMetrics: true
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable the collector |
| `namespaces` | []string | `[]` | Namespaces to watch (empty = all namespaces) |
| `resyncPeriod` | duration | `10m` | Informer resync interval |
| `includePhaseMetric` | bool | `true` | Include cluster phase metrics |
| `includeComponentMetrics` | bool | `true` | Include component status metrics |
| `includeConditionMetrics` | bool | `true` | Include condition metrics |

### Environment Variables

All configuration can be overridden using environment variables with the prefix `COLLECTORS_KUBEBLOCKS_`:

| Environment Variable | Maps To | Example |
|---------------------|---------|---------|
| `COLLECTORS_KUBEBLOCKS_ENABLED` | `enabled` | `true` |
| `COLLECTORS_KUBEBLOCKS_NAMESPACES` | `namespaces` | `ns-user1,ns-user2` |
| `COLLECTORS_KUBEBLOCKS_RESYNC_PERIOD` | `resyncPeriod` | `15m` |
| `COLLECTORS_KUBEBLOCKS_INCLUDE_PHASE_METRIC` | `includePhaseMetric` | `true` |
| `COLLECTORS_KUBEBLOCKS_INCLUDE_COMPONENT_METRICS` | `includeComponentMetrics` | `false` |
| `COLLECTORS_KUBEBLOCKS_INCLUDE_CONDITION_METRICS` | `includeConditionMetrics` | `false` |

## Metrics

### `sealos_kubeblocks_cluster_info`

**Type:** Gauge
**Labels:**
- `namespace`: Cluster namespace
- `cluster`: Cluster name
- `cluster_def`: ClusterDefinition reference
- `cluster_version`: ClusterVersion reference

**Description:** Informational metric about the cluster. Always `1`.

**Example:**
```promql
sealos_kubeblocks_cluster_info{namespace="ns-user1",cluster="my-postgres",cluster_def="postgresql",cluster_version="postgresql-14.8.0"} 1
```

### `sealos_kubeblocks_cluster_phase`

**Type:** Gauge
**Labels:**
- `namespace`: Cluster namespace
- `cluster`: Cluster name
- `phase`: Cluster phase (Creating, Running, Updating, Stopping, Stopped, Deleting, Failed, Abnormal)

**Description:** Cluster lifecycle phase. `1` indicates the current phase, `0` indicates not the current phase.

**Example:**
```promql
sealos_kubeblocks_cluster_phase{namespace="ns-user1",cluster="my-postgres",phase="Running"} 1
sealos_kubeblocks_cluster_phase{namespace="ns-user1",cluster="my-postgres",phase="Failed"} 0
sealos_kubeblocks_cluster_phase{namespace="ns-user1",cluster="my-postgres",phase="Creating"} 0
```

**Common Queries:**
```promql
# Clusters in Failed state
sealos_kubeblocks_cluster_phase{phase="Failed"} == 1

# Clusters not in Running state
sealos_kubeblocks_cluster_phase{phase="Running"} == 0
```

### `sealos_kubeblocks_cluster_component_phase`

**Type:** Gauge
**Labels:**
- `namespace`: Cluster namespace
- `cluster`: Cluster name
- `component`: Component name
- `phase`: Component phase (Creating, Running, Updating, Stopping, Stopped, Deleting, Failed, Abnormal)

**Description:** Component lifecycle phase. `1` indicates the current phase, `0` indicates not the current phase.

**Example:**
```promql
sealos_kubeblocks_cluster_component_phase{namespace="ns-user1",cluster="my-postgres",component="postgresql",phase="Running"} 1
sealos_kubeblocks_cluster_component_phase{namespace="ns-user1",cluster="my-postgres",component="postgresql",phase="Failed"} 0
```

### `sealos_kubeblocks_cluster_component_pods_ready`

**Type:** Gauge
**Labels:**
- `namespace`: Cluster namespace
- `cluster`: Cluster name
- `component`: Component name

**Description:** Component pods readiness status. `1` = all pods ready, `0` = not all pods ready.

**Example:**
```promql
sealos_kubeblocks_cluster_component_pods_ready{namespace="ns-user1",cluster="my-postgres",component="postgresql"} 1
```

**Common Queries:**
```promql
# Components with pods not ready
sealos_kubeblocks_cluster_component_pods_ready == 0
```

### `sealos_kubeblocks_cluster_condition`

**Type:** Gauge
**Labels:**
- `namespace`: Cluster namespace
- `cluster`: Cluster name
- `type`: Condition type (e.g., Ready, ProvisioningStarted, ApplyResources, ReplicasReady)
- `status`: Condition status (True, False, Unknown)
- `reason`: Condition reason

**Description:** Cluster condition status. `1` = condition is True, `0` = condition is False.

**Example:**
```promql
sealos_kubeblocks_cluster_condition{namespace="ns-user1",cluster="my-postgres",type="Ready",status="True",reason="ClusterReady"} 1
sealos_kubeblocks_cluster_condition{namespace="ns-user1",cluster="my-postgres",type="ReplicasReady",status="True",reason="AllReplicasReady"} 1
```

**Common Queries:**
```promql
# Clusters that are not ready
sealos_kubeblocks_cluster_condition{type="Ready",status="False"} == 1

# Clusters with provisioning issues
sealos_kubeblocks_cluster_condition{type="ProvisioningStarted",status="False"} == 1
```

### `sealos_kubeblocks_cluster_observed_generation`

**Type:** Gauge
**Labels:**
- `namespace`: Cluster namespace
- `cluster`: Cluster name

**Description:** The generation observed by the cluster controller. Used to detect if the controller has processed the latest spec changes.

**Example:**
```promql
sealos_kubeblocks_cluster_observed_generation{namespace="ns-user1",cluster="my-postgres"} 7
```

## Supported Resources

This collector monitors:
- **Group:** `apps.kubeblocks.io`
- **Version:** `v1alpha1`
- **Kind:** `Cluster`

## Architecture

The KubeBlocks collector uses a generic dynamic client controller framework:

1. **Dynamic Controller**: A reusable controller that watches any CRD using the Kubernetes dynamic client
2. **Event Handlers**: Callbacks for Add, Update, and Delete events
3. **Informer Cache**: Efficient caching and watching of resources with minimal API server load
4. **Multi-namespace Support**: Can watch specific namespaces or all namespaces cluster-wide

## Collector Type

**Type:** Informer-based
**Leader Election Required:** Yes

The KubeBlocks collector uses Kubernetes informers to efficiently watch cluster resources and only runs on the leader pod to avoid duplicate metrics.

## Performance Considerations

- **Memory**: The collector stores cluster status in memory. Memory usage scales with the number of monitored clusters.
- **API Calls**: Uses informers with watch connections, minimizing API server load.
- **Resync Period**: Default 10 minutes. Longer periods reduce API load but may delay detection of missed updates.
- **Namespace Filtering**: Monitoring specific namespaces reduces memory and API usage compared to cluster-wide monitoring.

## Example Alerting Rules

```yaml
groups:
  - name: kubeblocks_alerts
    interval: 30s
    rules:
      # Alert when cluster is in Failed state
      - alert: KubeBlocksClusterFailed
        expr: sealos_kubeblocks_cluster_phase{phase="Failed"} == 1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "KubeBlocks cluster {{ $labels.cluster }} in namespace {{ $labels.namespace }} has failed"
          description: "Cluster has been in Failed state for more than 5 minutes"

      # Alert when cluster is not ready
      - alert: KubeBlocksClusterNotReady
        expr: sealos_kubeblocks_cluster_condition{type="Ready",status="True"} == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "KubeBlocks cluster {{ $labels.cluster }} is not ready"
          description: "Cluster ready condition is not True for more than 5 minutes"

      # Alert when component pods are not ready
      - alert: KubeBlocksComponentPodsNotReady
        expr: sealos_kubeblocks_cluster_component_pods_ready == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "KubeBlocks component {{ $labels.component }} pods not ready"
          description: "Component pods have not been ready for more than 5 minutes"

      # Alert when cluster is stuck in Creating state
      - alert: KubeBlocksClusterStuckCreating
        expr: sealos_kubeblocks_cluster_phase{phase="Creating"} == 1
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "KubeBlocks cluster {{ $labels.cluster }} stuck in Creating state"
          description: "Cluster has been in Creating state for more than 30 minutes"
```

## Troubleshooting

### No metrics appearing

1. **Check if collector is enabled**: Verify `collectors.kubeblocks.enabled: true` in config
2. **Verify RBAC permissions**: The service account needs permissions to watch `clusters.apps.kubeblocks.io`
3. **Check logs**: Look for errors in the collector logs
4. **Leader election**: Ensure the pod is the leader (check logs for "became leader" message)

### Metrics missing for some clusters

1. **Namespace filtering**: If `namespaces` is configured, only clusters in those namespaces are monitored
2. **Cache sync**: Wait for the informer cache to sync (check "cache synced" in logs)
3. **Resource version**: Verify the KubeBlocks CRD version matches `v1alpha1`

### High memory usage

1. **Reduce monitored namespaces**: Specify only necessary namespaces instead of cluster-wide monitoring
2. **Disable unnecessary metrics**: Set `includeComponentMetrics` or `includeConditionMetrics` to `false`
3. **Increase resync period**: Longer resync periods reduce memory churn

## Required RBAC Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sealos-state-metric-kubeblocks
rules:
  - apiGroups: ["apps.kubeblocks.io"]
    resources: ["clusters"]
    verbs: ["get", "list", "watch"]
```

For namespace-scoped monitoring:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: sealos-state-metric-kubeblocks
  namespace: <namespace>
rules:
  - apiGroups: ["apps.kubeblocks.io"]
    resources: ["clusters"]
    verbs: ["get", "list", "watch"]
```
