# Zombie Collector

The Zombie collector detects zombie (defunct) processes running on Kubernetes nodes.

## Configuration

### YAML Configuration

```yaml
collectors:
  zombie:
    checkInterval: "30s"
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `checkInterval` | duration | `30s` | Interval between zombie process checks |

### Environment Variables

| Environment Variable | Maps To | Example |
|---------------------|---------|---------|
| `COLLECTORS_ZOMBIE_CHECK_INTERVAL` | `checkInterval` | `1m` |

## Metrics

### `sealos_zombie_processes_total`

**Type:** Gauge
**Labels:**
- `node`: Node name where zombie processes were detected
- `namespace`: Pod namespace
- `pod`: Pod name
- `container_id`: Container ID (short form)

**Description:** Total number of zombie processes detected in a container.

**Example:**
```promql
sealos_zombie_processes_total{node="worker-1",namespace="default",pod="app-abc",container_id="1a2b3c4d"} 3
sealos_zombie_processes_total{node="worker-2",namespace="kube-system",pod="monitor-xyz",container_id="5e6f7g8h"} 1
```

## What are Zombie Processes?

Zombie processes (defunct processes) are processes that have completed execution but still have an entry in the process table. They occur when:
- A child process terminates but the parent hasn't called `wait()` to read its exit status
- The parent process is poorly written or has bugs
- The parent process is stuck or unresponsive

While zombies don't consume system resources (CPU, memory), they:
- Occupy process table entries
- Can indicate application bugs
- May eventually exhaust PID limits if accumulated

## Use Cases

### Alerting on Zombie Processes

```promql
# Alert when any container has zombie processes
sealos_zombie_processes_total > 0

# Alert when zombie count exceeds threshold
sealos_zombie_processes_total > 5

# Alert on persistent zombies (present for >5 minutes)
sealos_zombie_processes_total > 0 and increase(sealos_zombie_processes_total[5m]) == 0
```

### Monitoring Zombie Trends

```promql
# Total zombies by node
sum by (node) (sealos_zombie_processes_total)

# Pods with most zombies
topk(10, sealos_zombie_processes_total)

# Namespaces affected by zombies
count by (namespace) (sealos_zombie_processes_total > 0)
```

## How It Works

The collector:
1. Reads `/proc` filesystem on each node
2. Identifies processes in 'Z' (zombie) state
3. Maps zombies to their parent processes
4. Correlates parent PIDs with container IDs using cgroups
5. Associates containers with their Kubernetes pods

## Requirements

- Must run with access to the host `/proc` filesystem
- Requires read permissions on `/proc/*`
- When running as DaemonSet, mount `/proc` from the host

## Collector Type

**Type:** Polling
**Leader Election Required:** No

The Zombie collector polls the process table at regular intervals to detect zombie processes.
