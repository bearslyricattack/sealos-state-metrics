package namespace

import (
	"maps"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/labring/sealos-state-metrics/pkg/collector/base"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// namespaceEventCounter stores event totals for one namespace. The pointer is
// retained after the namespace is deleted so a scrape can still observe a
// recently recorded event through increase().
type namespaceEventCounter struct {
	created atomic.Uint64
	updated atomic.Uint64
}

// Collector collects namespaces that are missing configured labels.
type Collector struct {
	*base.BaseCollector

	client   kubernetes.Interface
	config   *Config
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
	logger   *log.Entry

	mu         sync.RWMutex
	namespaces map[string]*corev1.Namespace
	whitelist  map[string]struct{}

	// dangerousNamespaceCreationCount counts Add events for namespaces that
	// were created without all required labels. It is intentionally retained
	// across collector restarts so a temporary scrape window does not hide an
	// observed creation.
	dangerousNamespaceCreationCount atomic.Uint64

	// requiredLabelChangeCount counts Update events where one or more required
	// labels changed on a non-whitelisted namespace.
	requiredLabelChangeCount atomic.Uint64

	// namespaceEventCounters contains per-namespace event totals. It is not
	// reset with the current namespace cache so event counters remain useful
	// after a namespace is deleted and across collector restarts.
	namespaceEventCounters map[string]*namespaceEventCounter

	// Metrics
	missingLabelsCount                   *prometheus.Desc
	missingLabelsInfo                    *prometheus.Desc
	missingLabelsCreatedTotal            *prometheus.Desc
	missingLabelsChangedTotal            *prometheus.Desc
	missingLabelsCreatedByNamespaceTotal *prometheus.Desc
	missingLabelsUpdatedByNamespaceTotal *prometheus.Desc
}

// initMetrics initializes Prometheus metric descriptors.
func (c *Collector) initMetrics(namespace string) {
	c.missingLabelsCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "namespace", "missing_labels_count"),
		"Number of namespaces missing one or more required labels",
		nil,
		nil,
	)
	c.missingLabelsInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "namespace", "missing_labels_info"),
		"Namespace missing one or more required labels (value is always 1)",
		[]string{"namespace", "missing_labels"},
		nil,
	)
	c.missingLabelsCreatedTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "namespace", "missing_labels_created_total"),
		"Total number of namespace creation events missing one or more required labels",
		nil,
		nil,
	)
	c.missingLabelsChangedTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "namespace", "missing_labels_changed_total"),
		"Total number of namespace update events changing one or more required labels",
		nil,
		nil,
	)
	c.missingLabelsCreatedByNamespaceTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "namespace", "missing_labels_created_by_namespace_total"),
		"Total number of namespace creation events missing required labels, by namespace",
		[]string{"namespace"},
		nil,
	)
	c.missingLabelsUpdatedByNamespaceTotal = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "namespace", "missing_labels_updated_by_namespace_total"),
		"Total number of namespace update events changing required labels, by namespace",
		[]string{"namespace"},
		nil,
	)

	c.MustRegisterDesc(c.missingLabelsCount)
	c.MustRegisterDesc(c.missingLabelsInfo)
	c.MustRegisterDesc(c.missingLabelsCreatedTotal)
	c.MustRegisterDesc(c.missingLabelsChangedTotal)
	c.MustRegisterDesc(c.missingLabelsCreatedByNamespaceTotal)
	c.MustRegisterDesc(c.missingLabelsUpdatedByNamespaceTotal)
}

// HasSynced returns true if the namespace informer has synced.
func (c *Collector) HasSynced() bool {
	return c.informer != nil && c.informer.HasSynced()
}

// collect emits the current count and one detail sample per affected
// namespace. Namespace names are sorted to keep scrape output deterministic.
func (c *Collector) collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	namespaces := make(map[string]*corev1.Namespace, len(c.namespaces))
	maps.Copy(namespaces, c.namespaces)
	eventCounters := make(map[string]*namespaceEventCounter, len(c.namespaceEventCounters))
	maps.Copy(eventCounters, c.namespaceEventCounters)
	c.mu.RUnlock()

	names := make([]string, 0, len(namespaces))
	for name := range namespaces {
		names = append(names, name)
	}

	sort.Strings(names)

	missingCount := 0
	for _, name := range names {
		if c.isWhitelisted(name) {
			continue
		}

		missingLabels := c.missingLabels(namespaces[name])
		if len(missingLabels) == 0 {
			continue
		}

		missingCount++

		ch <- prometheus.MustNewConstMetric(
			c.missingLabelsInfo,
			prometheus.GaugeValue,
			1,
			name,
			strings.Join(missingLabels, ","),
		)
	}

	ch <- prometheus.MustNewConstMetric(
		c.missingLabelsCount,
		prometheus.GaugeValue,
		float64(missingCount),
	)

	ch <- prometheus.MustNewConstMetric(
		c.missingLabelsCreatedTotal,
		prometheus.CounterValue,
		float64(c.dangerousNamespaceCreationCount.Load()),
	)

	ch <- prometheus.MustNewConstMetric(
		c.missingLabelsChangedTotal,
		prometheus.CounterValue,
		float64(c.requiredLabelChangeCount.Load()),
	)

	eventNames := make([]string, 0, len(eventCounters))
	for name := range eventCounters {
		if c.isWhitelisted(name) {
			continue
		}
		eventNames = append(eventNames, name)
	}
	sort.Strings(eventNames)

	for _, name := range eventNames {
		eventCounter := eventCounters[name]
		ch <- prometheus.MustNewConstMetric(
			c.missingLabelsCreatedByNamespaceTotal,
			prometheus.CounterValue,
			float64(eventCounter.created.Load()),
			name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.missingLabelsUpdatedByNamespaceTotal,
			prometheus.CounterValue,
			float64(eventCounter.updated.Load()),
			name,
		)
	}
}

// missingLabels returns configured labels that are absent from a namespace.
// A present label with an empty value still satisfies the requirement; the
// requirement is about label presence, not its value.
func (c *Collector) missingLabels(namespace *corev1.Namespace) []string {
	if namespace == nil {
		return nil
	}

	missing := make([]string, 0, len(c.config.RequiredLabels))
	for _, label := range c.config.RequiredLabels {
		if _, ok := namespace.Labels[label]; !ok {
			missing = append(missing, label)
		}
	}

	return missing
}
