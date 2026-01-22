# ImagePull Collector

The ImagePull collector monitors container image pull events and identifies slow or failed image pulls.

## Configuration

### YAML Configuration

```yaml
collectors:
  imagepull:
    slowPullThreshold: "5m"
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `slowPullThreshold` | duration | `5m` | Threshold for slow image pulls (pulls taking longer than this are reported) |

### Environment Variables

| Environment Variable | Maps To | Example |
|---------------------|---------|---------|
| `COLLECTORS_IMAGEPULL_SLOW_PULL_THRESHOLD` | `slowPullThreshold` | `10m` |

## Metrics

### `sealos_imagepull_duration_seconds`

**Type:** Gauge
**Labels:**
- `namespace`: Pod namespace
- `pod`: Pod name
- `container`: Container name
- `image`: Container image being pulled
- `node`: Node where the pull occurred

**Description:** Duration of the image pull operation in seconds. Only reported for pulls that exceed the `slowPullThreshold` or fail.

**Example:**
```promql
sealos_imagepull_duration_seconds{namespace="default",pod="web-abc",container="nginx",image="nginx:1.21",node="worker-1"} 420
sealos_imagepull_duration_seconds{namespace="default",pod="app-xyz",container="app",image="myapp:v2",node="worker-2"} 680
```

### `sealos_imagepull_failed`

**Type:** Gauge
**Labels:**
- `namespace`: Pod namespace
- `pod`: Pod name
- `container`: Container name
- `image`: Container image that failed to pull
- `node`: Node where the pull failed
- `reason`: Failure reason (e.g., `ImagePullBackOff`, `ErrImagePull`)

**Values:**
- `1`: Image pull failed
- `0`: Not used (failed pulls are removed when resolved)

**Example:**
```promql
sealos_imagepull_failed{namespace="default",pod="db-abc",container="postgres",image="postgres:14",node="worker-1",reason="ImagePullBackOff"} 1
sealos_imagepull_failed{namespace="kube-system",pod="monitor",container="prom",image="invalid:tag",node="worker-2",reason="ErrImagePull"} 1
```

## Use Cases

### Alerting on Image Pull Issues

```promql
# Alert on failed image pulls
sealos_imagepull_failed == 1

# Alert on slow image pulls (>5 minutes)
sealos_imagepull_duration_seconds > 300

# Alert on very slow pulls (>10 minutes)
sealos_imagepull_duration_seconds > 600
```

### Monitoring Image Pull Performance

```promql
# Average image pull duration by node
avg by (node) (sealos_imagepull_duration_seconds)

# Max image pull duration in last hour
max_over_time(sealos_imagepull_duration_seconds[1h])

# Count of failed pulls by image
count by (image) (sealos_imagepull_failed)
```

## Common Pull Failure Reasons

| Reason | Description |
|--------|-------------|
| `ImagePullBackOff` | Kubernetes is backing off on pulling the image after repeated failures |
| `ErrImagePull` | Error occurred while pulling the image |
| `InvalidImageName` | Image name format is invalid |
| `RegistryUnavailable` | Container registry is not accessible |

## Collector Type

**Type:** Informer
**Leader Election Required:** No

The ImagePull collector uses Kubernetes informers to watch pod events related to image pulling.
