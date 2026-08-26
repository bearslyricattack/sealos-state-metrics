package namespace

import (
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/labring/sealos-state-metrics/pkg/collector/base"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

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

	// Metrics
	missingLabelsCount *prometheus.Desc
	missingLabelsInfo  *prometheus.Desc
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

	c.MustRegisterDesc(c.missingLabelsCount)
	c.MustRegisterDesc(c.missingLabelsInfo)
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
	c.mu.RUnlock()

	names := make([]string, 0, len(namespaces))
	for name := range namespaces {
		names = append(names, name)
	}

	sort.Strings(names)

	missingCount := 0
	for _, name := range names {
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
