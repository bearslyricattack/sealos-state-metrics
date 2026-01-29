# Dynamic Collector Framework

The Dynamic Collector Framework provides TWO ways to monitor Kubernetes CRDs:

1. **Configuration-Driven Collector** (⭐ Recommended for most users): Monitor any CRD without writing code
2. **Programmatic Framework**: For advanced use cases requiring custom logic

## Configuration-Driven Collector

Monitor any Kubernetes CRD by simply adding configuration - no code required!

### Features

- **Zero code required**: Just add YAML configuration
- **Multiple CRDs**: Monitor multiple CRDs with a single collector
- **Rich metric types**: Info, state, gauge, map, and conditions
- **JSONPath field extraction**: Extract any field from your CRDs
- **Namespace filtering**: Watch specific namespaces or cluster-wide
- **Flexible labels**: Define custom labels for each metric

### Quick Example

```yaml
collectors:
  dynamic:
    enabled: true
    crds:
      - name: kubeblocks-cluster
        gvr:
          group: apps.kubeblocks.io
          version: v1alpha1
          resource: clusters
        commonLabels:
          cluster: metadata.name
          namespace: metadata.namespace
        metrics:
          - type: state
            name: phase
            help: "Cluster phase"
            path: status.phase
```

This configuration automatically creates metrics like:
```
# Only emits current state with value=1
sealos_kubeblocks_cluster_phase{cluster="my-db",namespace="default",state="Running"} 1
```

See [Configuration Guide](#configuration-guide) below for complete documentation.

---

## Configuration Guide

### Metric Types

The configuration-driven collector supports 6 metric types:

#### 1. `info` - Metadata Labels

Always emits value=1 with custom labels. Use for exposing metadata or state information.

```yaml
- type: info
  name: info
  help: "Resource information"
  labels:
    version: spec.version
    type: spec.type
```

Output:
```
resource_info{name="...", version="v1.0", type="app"} 1
```

**Can also be used for state**:
```yaml
- type: info
  name: phase
  help: "Resource phase"
  labels:
    phase: status.phase
```

Output:
```
resource_phase{name="app-1", phase="Running"} 1
resource_phase{name="app-2", phase="Pending"} 1
```

#### 2. `count` - Aggregate Count

Counts how many resources have each distinct value for a given field. Does not include per-resource labels - this is an aggregate metric.

```yaml
- type: count
  name: phase_count
  help: "Count of resources by phase"
  path: status.phase
  valueLabel: "phase"  # Optional, defaults to "value"
```

Output (aggregated):
```
resource_phase_count{phase="Running"} 5
resource_phase_count{phase="Pending"} 2
resource_phase_count{phase="Failed"} 1
```

**Use case**: Dashboards showing overall resource distribution by field value.

**Features**:
- Works with any field, not just states
- Customizable label name via `valueLabel`
- Efficient aggregation across all resources

#### 3. `gauge` - Numeric Value

Extracts a numeric value from each resource.

```yaml
- type: gauge
  name: replicas
  help: "Number of replicas"
  path: spec.replicas
```

Output:
```
resource_replicas{name="app-1"} 3
resource_replicas{name="app-2"} 5
```

#### 5. `map_state` - Map Entry States

Iterates over a map and emits the current state of each entry.

```yaml
- type: map_state
  name: component_phase
  help: "Component phase"
  path: status.components
  valuePath: phase
  keyLabel: component
```

Output:
```
resource_component_phase{name="app", component="mysql", state="Running"} 1
resource_component_phase{name="app", component="redis", state="Ready"} 1
```

#### 6. `map_gauge` - Map Entry Values

Iterates over a map and emits numeric values.

```yaml
- type: map_gauge
  name: component_replicas
  help: "Component replicas"
  path: status.components
  valuePath: replicas
  keyLabel: component
```

Output:
```
resource_component_replicas{name="app", component="mysql"} 3
resource_component_replicas{name="app", component="redis"} 2
```

#### 7. `conditions` - Kubernetes Conditions

Parses Kubernetes-style conditions (type, status, reason).

```yaml
- type: conditions
  name: condition
  help: "Resource conditions"
  path: status.conditions
```

Output:
```
resource_condition{name="app", type="Ready", status="True", reason="AllReady"} 1
resource_condition{name="app", type="Progressing", status="False", reason="Complete"} 0
```

### Common Configuration Fields

- `commonLabels`: Labels extracted for all metrics (except `state_count`)
- `namespaces`: List of namespaces to watch (empty = all)
- `resyncPeriod`: How often to resync with API server (default: 10m)

---

## Programmatic Framework

For advanced use cases requiring custom logic, the framework provides a reusable foundation for building collectors programmatically.

### Features

- **Generic informer management**: Handles CRD watching with efficient caching
- **Multi-namespace support**: Watch specific namespaces or cluster-wide
- **Event callbacks**: Simple Add/Update/Delete event handlers
- **Base collector integration**: Built on top of BaseCollector for lifecycle management
- **Leader election**: Ensures only one instance collects metrics
- **Minimal boilerplate**: Focus on business logic, not infrastructure

## Architecture

```
┌─────────────────────────────────────────┐
│         Your Custom Collector           │
│  (e.g., KubeBlocks, Crossplane, etc.)   │
│                                         │
│  - Configuration                        │
│  - Event Handlers (Add/Update/Delete)  │
│  - Metrics Collection Logic             │
│  - Metric Descriptors                   │
└─────────────────┬───────────────────────┘
                  │
                  │ Uses
                  ▼
┌─────────────────────────────────────────┐
│       Dynamic Collector Framework        │
│                                         │
│  - Informer Management                  │
│  - Multi-namespace Support              │
│  - Event Routing                        │
└─────────────────┬───────────────────────┘
                  │
                  │ Built on
                  ▼
┌─────────────────────────────────────────┐
│          Base Collector                 │
│                                         │
│  - Lifecycle Management (Start/Stop)   │
│  - Ready State Tracking                 │
│  - Leader Election                      │
│  - Prometheus Integration               │
└─────────────────────────────────────────┘
```

---

## 🔄 Complete Workflow

This section provides a detailed explanation of how the Dynamic Collector Framework operates from initialization to metric collection.

### Overview

The Dynamic Collector Framework follows a well-defined lifecycle with clear phases:

1. **Initialization**: Factory creates collector with configuration
2. **Registration**: Collector registers with registry and Prometheus
3. **Leader Election**: Only the leader pod starts actual collection
4. **Startup**: Controllers and informers are initialized
5. **Synchronization**: Informer caches sync with Kubernetes API
6. **Operation**: Events are processed and metrics are collected
7. **Shutdown**: Graceful cleanup of resources

### Phase 1: Initialization & Registration

#### 1.1 Factory Function Execution

```
User Code / Config
      │
      ▼
NewCollector() / NewConfigurableDynamicCollector()
      │
      ├─ Load configuration (YAML/env)
      ├─ Create dynamic client
      ├─ Initialize event handlers
      ├─ Create metric descriptors
      └─ Build Config struct
      │
      ▼
dynamic.NewCollector()
      │
      ├─ Validate configuration
      ├─ Create BaseCollector
      ├─ Register metric descriptors
      └─ Set lifecycle hooks
      │
      ▼
Return Collector instance
```

**Key Code Path** (Configuration-Driven):
```go
// pkg/collector/dynamic/factory.go:23-48
NewConfigurableDynamicCollector(factoryCtx)
  └─ LoadModuleConfig("collectors.dynamic", cfg)
  └─ createDynamicClient(restConfig)
  └─ newMultiCollector(cfg, dynamicClient, factoryCtx)
       └─ For each CRD:
            └─ NewConfigurableCollector(crdCfg, metricsNamespace, logger)
            └─ NewCollector(name, dynamicClient, dynamicConfig, logger)
```

#### 1.2 Configuration-Driven vs Programmatic

**Configuration-Driven Approach**:
```yaml
collectors:
  dynamic:
    crds:
      - name: my-crd
        gvr: {...}
        metrics: [...]
```
↓
```
ConfigurableCollector
  ├─ Parses YAML config
  ├─ Auto-generates event handlers
  ├─ Auto-generates metrics collectors
  └─ Uses generic field extraction
```

**Programmatic Approach**:
```go
impl := &MyCollectorImpl{...}
config := &dynamic.Config{
    EventHandler: impl.handlers,
    MetricsCollector: impl.collect,
    ...
}
```
↓
```
Custom Implementation
  ├─ Custom event handling logic
  ├─ Custom metrics collection
  ├─ Custom field extraction
  └─ Full control over behavior
```

### Phase 2: Leader Election

```
Multiple Pod Instances
      │
      ▼
Leader Election Process (pkg/leaderelection/)
      │
      ├─ Pod A: Becomes Leader ✓
      ├─ Pod B: Becomes Follower
      └─ Pod C: Becomes Follower
      │
      ▼
Only Leader Pod Proceeds
      │
      └─ Calls collector.Start(ctx)
```

**Non-leader pods**:
- Do NOT start informers
- Do NOT watch Kubernetes resources
- Do NOT collect metrics
- Wait in standby mode

**Leader pod**:
- Starts all informers
- Processes all events
- Collects and exposes metrics

### Phase 3: Collector Startup

#### 3.1 Start Call Chain

```
registry.Manager.Start(ctx)
      │
      ▼
collector.Start(ctx)  ← Your collector instance
      │
      ▼
BaseCollector.Start(ctx)
      │
      ├─ Create context with cancel
      ├─ Set started = true
      ├─ Create readyCh channel
      └─ Call lifecycle.OnStart(ctx)
            │
            ▼
      dynamic.Collector.start(ctx)  ← Dynamic collector implementation
            │
            ├─ Determine namespaces to watch
            ├─ Create Controller for each namespace
            │     │
            │     └─ controller.Start(ctx) ── See Phase 3.2 below
            │
            └─ collector.SetReady() ← Mark as ready after all synced
```

**Code Reference**: `pkg/collector/dynamic/collector.go:102-141`

#### 3.2 Controller Initialization

```
Controller.Start(ctx)  ← Per namespace
      │
      ├─ Create DynamicSharedInformerFactory
      │     ├─ Namespace-scoped: NewFilteredDynamicSharedInformerFactory()
      │     └─ Cluster-scoped: NewDynamicSharedInformerFactory()
      │
      ├─ Get informer for GVR: factory.ForResource(GVR).Informer()
      │
      ├─ Register event handlers:
      │     ├─ AddFunc    → EventHandler.OnAdd()
      │     ├─ UpdateFunc → EventHandler.OnUpdate()
      │     └─ DeleteFunc → EventHandler.OnDelete()
      │
      ├─ Start informer: go informer.Run(stopCh)
      │     │
      │     └─ Creates watch connection to API server
      │
      └─ Wait for cache sync: WaitForCacheSync()
            │
            └─ Blocks until initial list is cached
```

**Code Reference**: `pkg/collector/dynamic/controller.go:73-138`

### Phase 4: Informer Operation

#### 4.1 Kubernetes Informer Architecture

```
┌─────────────────────────────────────────────────────┐
│              Kubernetes API Server                   │
└────────────────────┬────────────────────────────────┘
                     │
                     │ Watch Connection (long-running HTTP)
                     ▼
┌─────────────────────────────────────────────────────┐
│                   Informer                           │
│                                                      │
│  ┌────────────────────────────────────────────┐   │
│  │           Local Cache (Store)               │   │
│  │  ┌──────────┬──────────┬──────────┐        │   │
│  │  │ Object 1 │ Object 2 │ Object 3 │  ...   │   │
│  │  └──────────┴──────────┴──────────┘        │   │
│  └────────────────────────────────────────────┘   │
│                                                      │
│  ┌────────────────────────────────────────────┐   │
│  │        Event Handler Queue                  │   │
│  │     [Add] [Update] [Delete] ...            │   │
│  └────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
                     │
                     │ Dequeues and dispatches events
                     ▼
┌─────────────────────────────────────────────────────┐
│           Your Event Handlers                        │
│  • OnAdd(obj)                                       │
│  • OnUpdate(oldObj, newObj)                         │
│  • OnDelete(obj)                                    │
└─────────────────────────────────────────────────────┘
```

#### 4.2 Event Processing Flow

**Initial Sync (on startup)**:
```
API Server
      │ Lists all existing resources
      ▼
Informer Cache
      │ Stores in local cache
      ▼
For each object:
      │ Triggers AddFunc
      ▼
EventHandler.OnAdd(obj)
      │
      └─ ConfigurableCollector.handleAdd(obj)
            │
            ├─ Extract fields using JSONPath
            ├─ Store in resources map
            └─ Log: "Resource added"
```

**Runtime Updates**:
```
Resource Changed in Cluster
      │
      ▼
API Server sends watch event
      │
      ▼
Informer receives event
      │
      ├─ Updates local cache
      └─ Triggers handler
            │
            ├─ ADDED event   → OnAdd(newObj)
            ├─ MODIFIED event → OnUpdate(oldObj, newObj)
            └─ DELETED event → OnDelete(oldObj)
```

**Code Reference**: `pkg/collector/dynamic/controller.go:99-121`

#### 4.3 ConfigurableCollector Event Handling

```
EventHandler.OnAdd(obj *unstructured.Unstructured)
      │
      ▼
ConfigurableCollector.handleAdd(obj)
      │
      ├─ Lock mutex (thread-safe)
      ├─ Generate key: namespace/name
      ├─ Store full object: resources[key] = obj
      ├─ Unlock mutex
      └─ Log debug message
```

**Key Points**:
- **Stores complete object**: Not just extracted fields, but the entire `unstructured.Unstructured`
- **Thread-safe**: Uses mutex to protect concurrent access
- **Memory-resident**: All watched resources are kept in memory
- **Update = Replace**: Update handler just calls Add to replace the object

**Code Reference**: `pkg/collector/dynamic/configurable.go:126-156`

### Phase 5: Metrics Collection

#### 5.1 Prometheus Collection Cycle

```
Prometheus Scrape Request
      │ HTTP GET /metrics
      ▼
Prometheus HTTP Handler
      │
      ▼
registry.Gather()
      │ Calls Describe() then Collect() on all collectors
      ▼
collector.Collect(ch chan<- prometheus.Metric)
      │
      ▼
BaseCollector.Collect(ch)
      │
      ├─ Check if WaitReadyOnCollect is enabled
      │     └─ If yes: WaitReady(ctx) with timeout
      │           ├─ Returns immediately if already ready
      │           └─ Blocks if not ready (up to timeout)
      │
      ├─ Check if ready
      │     └─ If not ready: Return early (no metrics)
      │
      └─ Call lifecycle.OnCollect(ch)
            │
            ▼
      dynamic.Collector (configured in factory)
            │
            └─ Calls config.MetricsCollector(ch)
                  │
                  ▼
            ConfigurableCollector.collect(ch)  ← See Phase 5.2
```

**Code Reference**: `pkg/collector/base/base.go` (Collect method)

#### 5.2 ConfigurableCollector Metrics Collection

```
ConfigurableCollector.collect(ch)
      │
      ├─ Acquire read lock (allow concurrent reads)
      │
      ├─ PASS 1: Per-Resource Metrics
      │     │
      │     └─ For each resource in resources map:
      │           │
      │           ├─ Extract common labels (namespace, name, etc.)
      │           │
      │           └─ For each metric config:
      │                 │
      │                 ├─ type: "info"
      │                 │     └─ collectInfoMetric()
      │                 │           └─ Emit 1.0 with metadata labels
      │                 │
      │                 ├─ type: "state"
      │                 │     └─ collectStateMetric()
      │                 │           ├─ Extract current state from path
      │                 │           └─ Emit 1.0 for current state only
      │                 │
      │                 ├─ type: "gauge"
      │                 │     └─ collectGaugeMetric()
      │                 │           ├─ Extract numeric value from path
      │                 │           └─ Emit value
      │                 │
      │                 ├─ type: "map_state"
      │                 │     └─ collectMapStateMetric()
      │                 │           ├─ Extract map from path
      │                 │           ├─ For each map entry:
      │                 │           │     ├─ Extract state from valuePath
      │                 │           │     └─ Emit 1.0 for current state
      │                 │           └─ Add keyLabel to labels
      │                 │
      │                 ├─ type: "map_gauge"
      │                 │     └─ collectMapGaugeMetric()
      │                 │           ├─ Extract map from path
      │                 │           └─ For each entry: emit value
      │                 │
      │                 └─ type: "conditions"
      │                       └─ collectConditionsMetric()
      │                             ├─ Extract conditions array
      │                             └─ For each condition:
      │                                   ├─ Extract type, status, reason
      │                                   └─ Emit 1.0 if status=True, 0.0 otherwise
      │
      └─ PASS 2: Aggregate Metrics
            │
            └─ For each "state_count" metric config:
                  │
                  ├─ Count resources by state across ALL resources
                  │     stateCounts := {"Running": 5, "Pending": 2, ...}
                  │
                  └─ For each state:
                        └─ Emit count with state label (no per-resource labels)
      │
      └─ Release read lock
```

**Example Output**:
```
# PASS 1: Per-resource metrics
kubeblocks_cluster_phase{cluster="db-1",namespace="prod",state="Running"} 1
kubeblocks_cluster_phase{cluster="db-2",namespace="prod",state="Pending"} 1
kubeblocks_cluster_observed_generation{cluster="db-1",namespace="prod"} 42

# PASS 2: Aggregate metrics
kubeblocks_cluster_phase_count{state="Running"} 5
kubeblocks_cluster_phase_count{state="Pending"} 2
kubeblocks_cluster_phase_count{state="Failed"} 1
```

**Code Reference**: `pkg/collector/dynamic/configurable.go:159-205`

#### 5.3 Field Extraction Mechanism

The framework uses JSONPath-like paths to extract fields from `unstructured.Unstructured` objects:

```
extractFieldString(obj, "status.phase")
      │
      ├─ Split path: ["status", "phase"]
      │
      ├─ Navigate object structure:
      │     obj.Object["status"] → map[string]any
      │     └─ ["phase"] → "Running"
      │
      └─ Return: "Running"
```

**Supported Paths**:
- Simple: `"metadata.name"` → `obj.GetName()`
- Nested: `"status.conditions[0].type"` → Navigate nested maps/slices
- Map access: `"status.components.mysql.phase"` → Access map by key

**Code Reference**: `pkg/collector/dynamic/utils.go`

### Phase 6: Resync Mechanism

#### 6.1 Periodic Resync

```
Informer (running in background)
      │
      ├─ Initial sync: Lists all resources
      │
      └─ Every ResyncPeriod (default: 10 minutes):
            │
            ├─ Re-lists ALL resources from API server
            │
            ├─ Compares with local cache
            │
            ├─ For each object:
            │     │
            │     ├─ If changed: Trigger UpdateFunc
            │     └─ If unchanged: No event fired
            │
            └─ Ensures eventual consistency
```

**Why Resync?**
- Recovers from missed watch events (network hiccup)
- Detects API server state drift
- Re-validates cached state periodically

**Configuration**:
```go
ControllerConfig{
    ResyncPeriod: 10 * time.Minute,  // Default
}
```

### Phase 7: Shutdown

#### 7.1 Graceful Shutdown Flow

```
SIGTERM received / Context cancelled
      │
      ▼
registry.Manager.Stop()
      │
      ▼
collector.Stop()
      │
      ▼
BaseCollector.Stop()
      │
      ├─ Check if already stopped
      ├─ Call lifecycle.OnStop()
      │     │
      │     ▼
      │   dynamic.Collector.stop()
      │     │
      │     └─ For each controller:
      │           │
      │           ▼
      │         controller.Stop()
      │           │
      │           ├─ Close informerStopCh
      │           │     │
      │           │     └─ Signals informer to stop
      │           │           │
      │           │           ├─ Closes watch connection
      │           │           └─ Stops event processing
      │           │
      │           └─ Log: "Stopping dynamic controller"
      │
      ├─ Cancel context (triggers cleanup)
      ├─ Set started = false
      └─ Close stoppedCh (signals waiting goroutines)
```

**Code Reference**:
- `pkg/collector/base/base.go` (Stop method)
- `pkg/collector/dynamic/collector.go:144-152`
- `pkg/collector/dynamic/controller.go:141-150`

### Phase 8: State Transitions

```
┌──────────┐
│ Created  │  ← NewCollector() returns
└─────┬────┘
      │ Start(ctx)
      ▼
┌──────────┐
│ Starting │  ← Initializing controllers
└─────┬────┘
      │ Informer cache synced
      ▼
┌──────────┐
│  Ready   │  ← SetReady() called, collecting metrics
└─────┬────┘
      │ Stop() or context cancelled
      ▼
┌──────────┐
│ Stopping │  ← Shutting down informers
└─────┬────┘
      │ Cleanup complete
      ▼
┌──────────┐
│ Stopped  │  ← Can be restarted
└──────────┘
```

**State Checks**:
- `collector.IsReady()`: Returns true only in "Ready" state
- `collector.WaitReady(ctx)`: Blocks until "Ready" or timeout
- `collector.Health()`: Checks if collector is healthy

### Metric Naming Convention

The framework constructs metric names as follows:

```
Metric Name Construction:

    <metricsNamespace>_<crdName>_<metricName>

    Example:
    metricsNamespace = "sealos"
    crdName = "kubeblocks-cluster"  → sanitized to "kubeblocks_cluster"
    metricName = "phase"

    Result: sealos_kubeblocks_cluster_phase

If metricsNamespace is empty:

    <crdName>_<metricName>

    Example: kubeblocks_cluster_phase
```

**Sanitization** (in `configurable.go:47-53`):
- Converts to lowercase
- Replaces hyphens with underscores
- Removes invalid characters
- Ensures valid Prometheus metric name

**Code Reference**: `pkg/collector/dynamic/configurable.go:45-53`

### Thread Safety

The framework ensures thread-safe operation through:

1. **Informer Queue**: Kubernetes client-go informers handle concurrent events internally
2. **Mutex Protection**: Event handlers lock before modifying shared state
3. **RWMutex for Collection**: Read lock allows concurrent metric collection
4. **Channel-based Communication**: Prometheus uses channels for metric emission

```go
type ConfigurableCollector struct {
    mu        sync.RWMutex  // Protects resources map
    resources map[string]*unstructured.Unstructured
}

func (c *ConfigurableCollector) handleAdd(obj *unstructured.Unstructured) {
    c.mu.Lock()         // Write lock
    defer c.mu.Unlock()
    c.resources[key] = obj
}

func (c *ConfigurableCollector) collect(ch chan<- prometheus.Metric) {
    c.mu.RLock()        // Read lock (allows concurrent reads)
    defer c.mu.RUnlock()
    // Read from resources map
}
```

### Performance Characteristics

**Memory Usage**:
- O(n) where n = number of watched resources
- Each resource stored as `unstructured.Unstructured` (~1-10 KB per object)
- Example: 1000 CRs × 5 KB = ~5 MB

**CPU Usage**:
- Low during steady state (just watch events)
- Spike during initial sync (processing all resources)
- Spike during Prometheus scrapes (iterating resources)

**Network Usage**:
- Initial list: Full resource retrieval
- Watch: Only delta events (very efficient)
- Resync: Full list every 10 minutes (configurable)

**Latency**:
- Event-to-metric latency: <100ms (in-memory processing)
- Initial ready time: Depends on cluster size (1s - 30s)

---

## Quick Start

### 1. Define Your Data Structure

```go
type MyResourceInfo struct {
    Namespace string
    Name      string
    Status    string
    // Add fields you want to track
}
```

### 2. Create Event Handlers

```go
type MyCollectorImpl struct {
    mu        sync.RWMutex
    resources map[string]*MyResourceInfo
}

func (impl *MyCollectorImpl) handleAdd(obj *unstructured.Unstructured) {
    info := impl.extractInfo(obj)

    impl.mu.Lock()
    defer impl.mu.Unlock()

    key := obj.GetNamespace() + "/" + obj.GetName()
    impl.resources[key] = info
}

func (impl *MyCollectorImpl) handleUpdate(oldObj, newObj *unstructured.Unstructured) {
    impl.handleAdd(newObj) // Often update is same as add
}

func (impl *MyCollectorImpl) handleDelete(obj *unstructured.Unstructured) {
    impl.mu.Lock()
    defer impl.mu.Unlock()

    key := obj.GetNamespace() + "/" + obj.GetName()
    delete(impl.resources, key)
}
```

### 3. Define Metrics Collection

```go
func (impl *MyCollectorImpl) collect(ch chan<- prometheus.Metric) {
    impl.mu.RLock()
    defer impl.mu.RUnlock()

    for _, resource := range impl.resources {
        ch <- prometheus.MustNewConstMetric(
            impl.statusDesc,
            prometheus.GaugeValue,
            1.0,
            resource.Namespace,
            resource.Name,
            resource.Status,
        )
    }
}
```

### 4. Create the Collector

```go
func NewMyCollector(factoryCtx *collector.FactoryContext) (collector.Collector, error) {
    // 1. Load configuration
    cfg := loadConfig()

    // 2. Create dynamic client
    dynamicClient, err := dynamic.NewForConfig(factoryCtx.RestConfig)
    if err != nil {
        return nil, err
    }

    // 3. Create implementation
    impl := &MyCollectorImpl{
        resources: make(map[string]*MyResourceInfo),
    }

    // Initialize metrics
    impl.statusDesc = prometheus.NewDesc(
        prometheus.BuildFQName(factoryCtx.MetricsNamespace, "myresource", "status"),
        "My resource status",
        []string{"namespace", "name", "status"},
        nil,
    )

    // 4. Configure dynamic collector
    dynamicConfig := &dynamic.Config{
        GVR: schema.GroupVersionResource{
            Group:    "myapi.example.com",
            Version:  "v1",
            Resource: "myresources",
        },
        Namespaces: cfg.Namespaces,
        EventHandler: dynamic.EventHandlerFuncs{
            AddFunc:    impl.handleAdd,
            UpdateFunc: impl.handleUpdate,
            DeleteFunc: impl.handleDelete,
        },
        MetricsCollector: impl.collect,
        MetricDescriptors: []*prometheus.Desc{impl.statusDesc},
    }

    // 5. Create dynamic collector
    return dynamic.NewCollector(
        "myresource",
        dynamicClient,
        dynamicConfig,
        factoryCtx.Logger,
    )
}
```

## API Reference

### Types

#### `Config`

Configuration for a dynamic collector:

```go
type Config struct {
    // GVR is the GroupVersionResource to watch
    GVR schema.GroupVersionResource

    // Namespaces to watch (empty slice means all namespaces)
    Namespaces []string

    // EventHandler is the callback interface for resource events
    EventHandler EventHandler

    // MetricsCollector is the function to collect metrics
    MetricsCollector func(ch chan<- prometheus.Metric)

    // MetricDescriptors are the Prometheus metric descriptors to register
    MetricDescriptors []*prometheus.Desc
}
```

#### `EventHandler`

Interface for handling resource events:

```go
type EventHandler interface {
    OnAdd(obj *unstructured.Unstructured)
    OnUpdate(oldObj, newObj *unstructured.Unstructured)
    OnDelete(obj *unstructured.Unstructured)
}
```

Helper implementation:

```go
type EventHandlerFuncs struct {
    AddFunc    func(obj *unstructured.Unstructured)
    UpdateFunc func(oldObj, newObj *unstructured.Unstructured)
    DeleteFunc func(obj *unstructured.Unstructured)
}
```

### Functions

#### `NewCollector`

Creates a new dynamic collector:

```go
func NewCollector(
    name string,
    dynamicClient dynamic.Interface,
    config *Config,
    logger *log.Entry,
    opts ...base.BaseCollectorOption,
) (*Collector, error)
```

Parameters:
- `name`: Collector name (used in logs and metrics)
- `dynamicClient`: Kubernetes dynamic client
- `config`: Collector configuration
- `logger`: Logger instance
- `opts`: Optional BaseCollector options

## Features

### Multi-Namespace Support

Watch specific namespaces:

```go
config := &dynamic.Config{
    Namespaces: []string{"ns-user1", "ns-user2"},
    // ...
}
```

Or watch all namespaces:

```go
config := &dynamic.Config{
    Namespaces: []string{}, // Empty = all namespaces
    // ...
}
```

### Leader Election

The collector automatically participates in leader election. Only the leader pod will run the informers and collect metrics.

### Efficient Watching

Uses Kubernetes informers for efficient resource watching:
- Watches use long-running connections to the API server
- Local caching minimizes API calls
- Automatic resync ensures consistency

### Lifecycle Management

Inherits lifecycle management from BaseCollector:
- `Start()`: Initializes informers and begins watching
- `Stop()`: Gracefully stops all informers
- `IsReady()`: Indicates when the cache is synced and ready

## Examples

### KubeBlocks Collector

See `pkg/collector/kubeblocks/` for a complete example of using the dynamic collector framework to monitor KubeBlocks Cluster resources.

Key files:
- `collector.go`: Implementation with event handlers and metrics collection
- `factory.go`: Factory function that creates the dynamic collector
- `config.go`: Configuration structure

## Best Practices

### 1. Thread-Safe State Management

Always protect shared state with mutexes:

```go
type MyImpl struct {
    mu    sync.RWMutex
    state map[string]*Info
}

func (impl *MyImpl) handleAdd(obj *unstructured.Unstructured) {
    impl.mu.Lock()
    defer impl.mu.Unlock()
    // Modify state
}

func (impl *MyImpl) collect(ch chan<- prometheus.Metric) {
    impl.mu.RLock() // Read lock for collection
    defer impl.mu.RUnlock()
    // Read state and emit metrics
}
```

### 2. Graceful Error Handling

Don't panic in event handlers:

```go
func (impl *MyImpl) handleAdd(obj *unstructured.Unstructured) {
    info, err := impl.extractInfo(obj)
    if err != nil {
        impl.logger.WithError(err).Warn("Failed to extract info")
        return // Don't crash, just skip
    }
    // Process info
}
```

### 3. Efficient Metric Collection

Pre-create metric descriptors:

```go
// In initialization
impl.statusDesc = prometheus.NewDesc(...)

// In collect
ch <- prometheus.MustNewConstMetric(
    impl.statusDesc, // Reuse descriptor
    prometheus.GaugeValue,
    value,
    labels...,
)
```

### 4. Namespace Filtering

For multi-tenant environments, allow namespace filtering:

```go
type Config struct {
    Namespaces []string `yaml:"namespaces"`
}

// In factory
config := &dynamic.Config{
    Namespaces: cfg.Namespaces,
    // ...
}
```

## Performance Considerations

### Memory Usage

- The collector stores resource state in memory
- Memory usage scales with: (number of resources) × (size of extracted info)
- Use namespace filtering to reduce memory footprint

### API Server Load

- Informers use watch connections (minimal load)
- Resync period affects API load (default: 10 minutes)
- Longer resync = less load but slower to detect missed events

### Metrics Cardinality

- Be careful with high-cardinality labels (e.g., pod names)
- Prefer lower-cardinality labels (e.g., phase, status)
- Consider aggregating data before exposing metrics

## 🔍 Debugging & Troubleshooting

### Common Issues

#### No metrics appearing

**Symptoms**: `/metrics` endpoint returns no data for your collector

**Diagnostic Steps**:

1. **Check if collector is registered**:
   ```bash
   # Look for collector registration in logs
   grep "Registered collector" /path/to/logs
   ```

2. **Verify leader election**:
   ```bash
   # Check if this pod is the leader
   grep "became leader" /path/to/logs
   # Or check the lease
   kubectl get lease -n <namespace> <lease-name> -o yaml
   ```

   - Only the leader pod collects metrics
   - Non-leader pods will NOT export any metrics
   - Check `requiresLeaderElection` setting

3. **Check cache sync**:
   ```bash
   # Look for cache sync messages
   grep "cache synced" /path/to/logs
   grep "Dynamic controller started" /path/to/logs
   ```

   - If stuck syncing: Check API server connectivity
   - If "failed to sync cache": Increase timeout or check RBAC

4. **Verify RBAC permissions**:
   ```bash
   # Check if ServiceAccount can watch the resource
   kubectl auth can-i watch <resource> --as=system:serviceaccount:<ns>:<sa>
   kubectl auth can-i list <resource> --as=system:serviceaccount:<ns>:<sa>
   ```

   Required permissions:
   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   rules:
   - apiGroups: ["your.api.group"]
     resources: ["yourresources"]
     verbs: ["get", "list", "watch"]
   ```

5. **Check collector is ready**:
   ```bash
   # Access health endpoint
   curl http://localhost:8080/healthz
   curl http://localhost:8080/ready
   ```

6. **Enable debug logging**:
   ```bash
   # Set log level to debug
   export LOG_LEVEL=debug
   ```

   Look for:
   - "Resource added" messages (events being processed)
   - "Collecting metrics" messages
   - Any error messages

#### Metrics show stale data

**Symptoms**: Metrics don't update when resources change

**Causes & Solutions**:

1. **Informer not receiving events**:
   - Check watch connection: `grep "watch" /path/to/logs`
   - Network issues between pod and API server
   - API server may have dropped the watch

2. **Event handler errors**:
   - Check for errors in OnAdd/OnUpdate/OnDelete handlers
   - Look for "Failed to extract" or similar errors
   - Fix field extraction paths

3. **Cache out of sync**:
   - Wait for next resync (default: 10 minutes)
   - Restart the collector pod to force re-sync
   - Reduce `resyncPeriod` if needed

#### High memory usage

**Symptoms**: Collector consuming excessive memory

**Analysis**:

```bash
# Check number of resources
kubectl get <resource> --all-namespaces | wc -l

# Check memory usage
kubectl top pod <collector-pod>
```

**Solutions**:

1. **Enable namespace filtering**:
   ```yaml
   namespaces: ["ns1", "ns2"]  # Instead of watching all
   ```

2. **Reduce stored data** (Programmatic approach):
   ```go
   // Instead of storing full object
   type MyInfo struct {
       Name   string  // Only store what you need
       Status string
   }
   ```

3. **Limit watched resources**:
   - Use label selectors (if supported by informer)
   - Split into multiple collectors by namespace

4. **Monitor and tune**:
   - Set memory limits in pod spec
   - Monitor OOM kills
   - Adjust based on cluster size

#### Slow startup / Ready timeout

**Symptoms**: Collector takes long time to become ready

**Causes**:

1. **Large resource count**: Initial sync loads all resources
   ```bash
   # Check resource count
   kubectl get <resource> --all-namespaces -o json | jq '.items | length'
   ```

2. **API server throttling**: Rate limits on API requests
   ```bash
   # Check for throttling in metrics
   curl localhost:8080/metrics | grep throttle
   ```

3. **Network latency**: Slow connection to API server

**Solutions**:

1. **Increase wait-ready timeout**:
   ```go
   opts := []base.BaseCollectorOption{
       base.WithWaitReadyTimeout(30 * time.Second),  // Increase from default 5s
   }
   ```

2. **Reduce initial sync load**:
   - Use namespace filtering
   - Consider pagination (for very large clusters)

3. **Optimize resync period**:
   ```go
   ResyncPeriod: 30 * time.Minute,  // Reduce frequency
   ```

#### Metrics missing for some resources

**Symptoms**: Some resources appear in `kubectl get` but not in metrics

**Debug Steps**:

1. **Check event handler logs**:
   ```bash
   # Look for "Resource added" for the missing resource
   grep "namespace/name" /path/to/logs
   ```

2. **Verify field extraction**:
   - Resource may not have the expected field
   - Check if `path` in metric config is correct

   ```bash
   # Get resource YAML to verify structure
   kubectl get <resource> <name> -n <namespace> -o yaml
   ```

3. **Check for extraction errors**:
   ```go
   // In ConfigurableCollector, check for empty values
   value := extractFieldString(obj, cfg.Path)
   if value == "" {
       return  // Metric not emitted
   }
   ```

4. **Validate metric conditions**:
   - For `state` metrics: Resource must have a state value
   - For `map_*` metrics: Map must exist and have entries
   - For `conditions`: Conditions array must be present

#### Duplicate metrics

**Symptoms**: Same metric emitted multiple times

**Causes**:

1. **Multiple collectors watching same resource**:
   - Check if you have duplicate collector configurations
   - Ensure only one collector per CRD

2. **Multiple namespaces** (expected behavior):
   - If watching multiple namespaces, you'll get metrics for each
   - This is normal if resources exist in multiple namespaces

3. **Informer duplication bug**:
   - Restart collector pod
   - Check for controller creation errors in logs

### Advanced Debugging

#### Enable Prometheus Debug Endpoint

```bash
# Check registered metrics
curl http://localhost:8080/metrics | grep your_metric_name

# Check metric descriptors
curl http://localhost:8080/debug/metrics
```

#### Inspect Informer Cache

```go
// For programmatic collectors, you can expose cache contents
func (c *Controller) DumpCache() {
    store := c.informer.GetStore()
    for _, obj := range store.List() {
        log.Printf("Cached object: %+v", obj)
    }
}
```

#### Monitor Event Processing

```go
// Add custom logging to event handlers
func (impl *MyImpl) handleAdd(obj *unstructured.Unstructured) {
    log.WithFields(log.Fields{
        "namespace": obj.GetNamespace(),
        "name":      obj.GetName(),
        "rv":        obj.GetResourceVersion(),
    }).Debug("Processing Add event")

    // Your logic here
}
```

#### Trace Metric Collection

```bash
# Time how long collection takes
curl -w "\nTime: %{time_total}s\n" http://localhost:8080/metrics > /dev/null

# Check Prometheus scrape duration
curl http://localhost:8080/metrics | grep scrape_duration
```

### Performance Tuning

#### Optimize for Large Clusters

1. **Batch processing** (for custom collectors):
   ```go
   func (impl *MyImpl) collect(ch chan<- prometheus.Metric) {
       impl.mu.RLock()
       defer impl.mu.RUnlock()

       // Pre-allocate slices
       metrics := make([]prometheus.Metric, 0, len(impl.resources))

       // Collect first, emit later
       for _, resource := range impl.resources {
           metric := impl.buildMetric(resource)
           metrics = append(metrics, metric)
       }

       // Emit all at once
       for _, m := range metrics {
           ch <- m
       }
   }
   ```

2. **Reduce lock contention**:
   ```go
   // Copy data under lock, process outside lock
   func (impl *MyImpl) collect(ch chan<- prometheus.Metric) {
       impl.mu.RLock()
       resourcesCopy := make([]*Info, 0, len(impl.resources))
       for _, r := range impl.resources {
           resourcesCopy = append(resourcesCopy, r)
       }
       impl.mu.RUnlock()

       // Process without holding lock
       for _, r := range resourcesCopy {
           ch <- impl.buildMetric(r)
       }
   }
   ```

3. **Optimize resync period**:
   - Default: 10 minutes
   - Smaller clusters: Can use shorter period (5 minutes)
   - Large clusters: Use longer period (30 minutes)
   - Critical metrics: Shorter period
   - Stable resources: Longer period

4. **Namespace filtering**:
   ```yaml
   # Watch only relevant namespaces
   collectors:
     dynamic:
       crds:
         - name: my-crd
           namespaces:
             - prod-*      # If pattern matching supported
             - staging-*
   ```

### Monitoring the Collector

Add observability to your collector:

1. **Collector-specific metrics**:
   ```go
   var (
       resourcesWatched = prometheus.NewGauge(prometheus.GaugeOpts{
           Name: "collector_resources_watched_total",
           Help: "Number of resources currently watched",
       })

       eventsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
           Name: "collector_events_processed_total",
           Help: "Number of events processed by type",
       }, []string{"event_type"})
   )

   func (impl *MyImpl) handleAdd(obj *unstructured.Unstructured) {
       eventsProcessed.WithLabelValues("add").Inc()
       // ... rest of handler
   }
   ```

2. **Health checks**:
   ```go
   func (c *Collector) Health() error {
       if !c.IsReady() {
           return errors.New("collector not ready")
       }

       // Check if informer is healthy
       for _, ctrl := range c.controllers {
           if !ctrl.HasSynced() {
               return errors.New("informer cache not synced")
           }
       }

       return nil
   }
   ```

3. **Structured logging**:
   ```go
   c.logger.WithFields(log.Fields{
       "resources": len(impl.resources),
       "duration":  time.Since(start),
   }).Info("Metrics collection completed")
   ```

## 📊 Workflow Diagrams

### Configuration-Driven Complete Flow

```text
User provides YAML configuration
         ↓
┌────────────────────────────────────────────────┐
│ 1. INITIALIZATION PHASE                        │
│    • Parse YAML config                         │
│    • Create ConfigurableCollector              │
│    • Auto-generate event handlers              │
│    • Auto-generate metrics collectors          │
│    • Register metric descriptors               │
└────────────────┬───────────────────────────────┘
         ↓
┌────────────────────────────────────────────────┐
│ 2. LEADER ELECTION                             │
│    • Multiple pods compete                     │
│    • One becomes leader                        │
│    • Only leader proceeds                      │
└────────────────┬───────────────────────────────┘
         ↓
┌────────────────────────────────────────────────┐
│ 3. STARTUP PHASE                               │
│    • Create Controllers (one per namespace)    │
│    • Create Informers                          │
│    • Register event handlers                   │
│    • Start watch connections                   │
│    • Initial sync from API server              │
└────────────────┬───────────────────────────────┘
         ↓
┌────────────────────────────────────────────────┐
│ 4. OPERATIONAL PHASE                           │
│                                                │
│  ┌──────────────┐      ┌─────────────────┐   │
│  │   K8s API    │─────▶│    Informer     │   │
│  │   Server     │ Watch │   Cache         │   │
│  └──────────────┘ Event└────────┬────────┘   │
│                                   │            │
│                         Add/Update/Delete      │
│                                   ↓            │
│                         ┌─────────────────┐   │
│                         │ Event Handler   │   │
│                         │ • Extract fields│   │
│                         │ • Store in map  │   │
│                         └─────────────────┘   │
│                                                │
│  ┌──────────────┐      ┌─────────────────┐   │
│  │  Prometheus  │─────▶│ Collect Metrics │   │
│  │   Scrape     │ GET  │ • Read from map │   │
│  └──────────────┘      │ • Emit metrics  │   │
│                         └─────────────────┘   │
└────────────────────────────────────────────────┘
         ↓
┌────────────────────────────────────────────────┐
│ 5. SHUTDOWN PHASE                              │
│    • Stop informers                            │
│    • Close watch connections                   │
│    • Clean up resources                        │
└────────────────────────────────────────────────┘
```

### Event Processing Timeline

```text
Time →

t0: Resource created in cluster
    │
    ├─ ~50ms: API server processes
    ├─ ~100ms: Watch event sent
    │
t+100ms: Informer receives event
    │
    ├─ Update local cache
    ├─ Queue event for processing
    │
t+120ms: Event handler called
    │
    ├─ OnAdd(obj)
    ├─ Extract fields
    ├─ Store in map
    │
t+130ms: Event processing complete
    │
    ... (resource stored in memory)
    │
t+1000ms: Prometheus scrape arrives
    │
    ├─ Collect() called
    ├─ Iterate resources map
    ├─ Emit metrics
    │
t+1050ms: Metrics returned to Prometheus
```

## 📚 Best Practices Summary

### DO ✅

1. **Use Configuration-Driven approach** for simple CRD monitoring
2. **Enable namespace filtering** in multi-tenant environments
3. **Use appropriate metric types** (state, state_count, gauge, info)
4. **Protect shared state with mutexes** (write lock for modifications, read lock for collection)
5. **Handle errors gracefully** (don't panic, log and continue)
6. **Pre-create metric descriptors** (create once, reuse during collection)
7. **Use structured logging** with relevant context
8. **Test your collector** with unit and integration tests

### DON'T ❌

1. **Don't block in event handlers** (keep handlers fast <10ms)
2. **Don't store unnecessary data** (only keep fields needed for metrics)
3. **Don't ignore ready state** (wait for cache sync before collecting)
4. **Don't create high-cardinality labels** (avoid pod names, UUIDs)
5. **Don't modify objects in event handlers** (informer cache is read-only)
6. **Don't assume field existence** (always check, handle nil values)
7. **Don't leak goroutines** (ensure proper cleanup in Stop())
8. **Don't log excessively** (use Debug level for verbose logs)

## Contributing

When creating a new collector using this framework:

1. Create a new package under `pkg/collector/yourname/`
2. Implement event handlers and metrics collection
3. Create a factory function that configures the dynamic collector
4. Register your collector in `init()`
5. Add documentation and examples
6. Write tests for event handling and metric collection
7. Document configuration options
8. Add README with usage examples

See `pkg/collector/kubeblocks/` for a reference implementation.

## License

See the main project LICENSE file.
