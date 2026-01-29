package dynamic

import (
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ConfigurableCollector implements a configuration-driven CRD collector
type ConfigurableCollector struct {
	logger       *log.Entry
	crdConfig    *CRDConfig
	metricPrefix string

	mu        sync.RWMutex
	resources map[string]*unstructured.Unstructured // key: namespace/name

	// Metric descriptors
	descriptors map[string]*prometheus.Desc
}

// NewConfigurableCollector creates a new configurable collector for a CRD
func NewConfigurableCollector(
	crdConfig *CRDConfig,
	metricPrefix string,
	logger *log.Entry,
) *ConfigurableCollector {
	c := &ConfigurableCollector{
		logger:       logger,
		crdConfig:    crdConfig,
		metricPrefix: metricPrefix,
		resources:    make(map[string]*unstructured.Unstructured),
		descriptors:  make(map[string]*prometheus.Desc),
	}

	c.initMetrics()

	return c
}

// initMetrics initializes Prometheus metric descriptors
func (c *ConfigurableCollector) initMetrics() {
	// Build prefix: either "prefix_crdname" or just "crdname" if prefix is empty
	var prefix string
	if c.metricPrefix != "" {
		prefix = fmt.Sprintf("%s_%s", c.metricPrefix, sanitizeName(c.crdConfig.Name))
	} else {
		prefix = sanitizeName(c.crdConfig.Name)
	}

	// Get common label names
	commonLabelNames := c.getCommonLabelNames()

	for _, metricCfg := range c.crdConfig.Metrics {
		var labelNames []string

		metricName := prometheus.BuildFQName(prefix, "", metricCfg.Name)

		switch metricCfg.Type {
		case "info":
			// Info metrics have common labels + extra labels
			labelNames = append(labelNames, commonLabelNames...)
			labelNames = append(labelNames, getSortedKeys(metricCfg.Labels)...)

		case "count":
			// Count metrics are aggregate metrics that count resources by a field value
			// Only has the value label (no per-resource labels)
			valueLabel := metricCfg.ValueLabel
			if valueLabel == "" {
				valueLabel = "value" // Default label name
			}

			labelNames = []string{valueLabel}

		case "gauge":
			// Gauge metrics have only common labels
			labelNames = commonLabelNames

		case "map_state":
			// Map state metrics have common labels + key label + state label
			labelNames = append(labelNames, commonLabelNames...)
			labelNames = append(labelNames, metricCfg.KeyLabel, "state")

		case "map_gauge":
			// Map gauge metrics have common labels + key label
			labelNames = append(labelNames, commonLabelNames...)
			labelNames = append(labelNames, metricCfg.KeyLabel)

		case "conditions":
			// Condition metrics have common labels + type, status, reason
			labelNames = append(labelNames, commonLabelNames...)
			labelNames = append(labelNames, "type", "status", "reason")

		default:
			c.logger.WithField("type", metricCfg.Type).Warn("Unknown metric type")
			continue
		}

		desc := prometheus.NewDesc(metricName, metricCfg.Help, labelNames, nil)
		c.descriptors[metricCfg.Name] = desc
	}
}

// getCommonLabelNames returns sorted common label names
func (c *ConfigurableCollector) getCommonLabelNames() []string {
	return getSortedKeys(c.crdConfig.CommonLabels)
}

// GetMetricDescriptors returns all metric descriptors
func (c *ConfigurableCollector) GetMetricDescriptors() []*prometheus.Desc {
	descs := make([]*prometheus.Desc, 0, len(c.descriptors))
	for _, desc := range c.descriptors {
		descs = append(descs, desc)
	}

	return descs
}

// GetEventHandler returns the event handler
func (c *ConfigurableCollector) GetEventHandler() EventHandler {
	return EventHandlerFuncs{
		AddFunc:    c.handleAdd,
		UpdateFunc: c.handleUpdate,
		DeleteFunc: c.handleDelete,
	}
}

// GetMetricsCollector returns the metrics collector function
func (c *ConfigurableCollector) GetMetricsCollector() func(ch chan<- prometheus.Metric) {
	return c.collect
}

// handleAdd processes add events
func (c *ConfigurableCollector) handleAdd(obj *unstructured.Unstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := obj.GetNamespace() + "/" + obj.GetName()
	c.resources[key] = obj

	c.logger.WithFields(log.Fields{
		"namespace": obj.GetNamespace(),
		"name":      obj.GetName(),
	}).Debug("Resource added")
}

// handleUpdate processes update events
func (c *ConfigurableCollector) handleUpdate(oldObj, newObj *unstructured.Unstructured) {
	c.handleAdd(newObj)
}

// handleDelete processes delete events
func (c *ConfigurableCollector) handleDelete(obj *unstructured.Unstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := obj.GetNamespace() + "/" + obj.GetName()
	delete(c.resources, key)

	c.logger.WithFields(log.Fields{
		"namespace": obj.GetNamespace(),
		"name":      obj.GetName(),
	}).Debug("Resource deleted")
}

// collect collects metrics
func (c *ConfigurableCollector) collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// First pass: collect per-resource metrics
	for _, obj := range c.resources {
		// Get common labels
		commonLabels := c.extractCommonLabels(obj)

		// Collect each configured metric
		for _, metricCfg := range c.crdConfig.Metrics {
			desc, ok := c.descriptors[metricCfg.Name]
			if !ok {
				continue
			}

			switch metricCfg.Type {
			case "info":
				c.collectInfoMetric(ch, desc, obj, &metricCfg, commonLabels)
			case "gauge":
				c.collectGaugeMetric(ch, desc, obj, &metricCfg, commonLabels)
			case "map_state":
				c.collectMapStateMetric(ch, desc, obj, &metricCfg, commonLabels)
			case "map_gauge":
				c.collectMapGaugeMetric(ch, desc, obj, &metricCfg, commonLabels)
			case "conditions":
				c.collectConditionsMetric(ch, desc, obj, &metricCfg, commonLabels)
			}
		}
	}

	// Second pass: collect aggregate metrics (count)
	for _, metricCfg := range c.crdConfig.Metrics {
		if metricCfg.Type != "count" {
			continue
		}

		desc, ok := c.descriptors[metricCfg.Name]
		if !ok {
			continue
		}

		c.collectCountMetric(ch, desc, &metricCfg)
	}
}

// extractCommonLabels extracts common labels from an object
func (c *ConfigurableCollector) extractCommonLabels(obj *unstructured.Unstructured) []string {
	labels := make([]string, 0, len(c.crdConfig.CommonLabels))

	for _, path := range getSortedValues(c.crdConfig.CommonLabels) {
		value := extractFieldString(obj, path)
		labels = append(labels, value)
	}

	return labels
}

// collectInfoMetric collects an info metric
func (c *ConfigurableCollector) collectInfoMetric(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	obj *unstructured.Unstructured,
	cfg *MetricConfig,
	commonLabels []string,
) {
	labels := make([]string, len(commonLabels), len(commonLabels)+len(cfg.Labels))
	copy(labels, commonLabels)

	// Add extra labels
	for _, path := range getSortedValues(cfg.Labels) {
		value := extractFieldString(obj, path)
		labels = append(labels, value)
	}

	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, labels...)
}

// collectCountMetric collects count metrics (aggregate)
// Counts how many resources have each distinct value for a given field across all resources
func (c *ConfigurableCollector) collectCountMetric(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	cfg *MetricConfig,
) {
	// Count resources by field value
	valueCounts := make(map[string]float64)

	for _, obj := range c.resources {
		value := extractFieldString(obj, cfg.Path)
		if value != "" {
			valueCounts[value]++
		}
	}

	// Emit metrics for each discovered value
	for value, count := range valueCounts {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, count, value)
	}
}

// collectGaugeMetric collects a gauge metric
func (c *ConfigurableCollector) collectGaugeMetric(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	obj *unstructured.Unstructured,
	cfg *MetricConfig,
	commonLabels []string,
) {
	value := extractFieldFloat(obj, cfg.Path)

	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, commonLabels...)
}

// collectMapStateMetric collects a map state metric
// Only emits the current state for each map entry with value=1
func (c *ConfigurableCollector) collectMapStateMetric(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	obj *unstructured.Unstructured,
	cfg *MetricConfig,
	commonLabels []string,
) {
	mapData := extractFieldMap(obj, cfg.Path)

	for key, entryData := range mapData {
		entryMap, ok := entryData.(map[string]any)
		if !ok {
			continue
		}

		currentState, _ := entryMap[cfg.ValuePath].(string)

		// Only emit metric if there is a current state
		if currentState == "" {
			continue
		}

		labels := make([]string, len(commonLabels), len(commonLabels)+2)
		copy(labels, commonLabels)
		labels = append(labels, key, currentState)

		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1.0, labels...)
	}
}

// collectMapGaugeMetric collects a map gauge metric
func (c *ConfigurableCollector) collectMapGaugeMetric(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	obj *unstructured.Unstructured,
	cfg *MetricConfig,
	commonLabels []string,
) {
	mapData := extractFieldMap(obj, cfg.Path)

	for key, entryData := range mapData {
		entryMap, ok := entryData.(map[string]any)
		if !ok {
			continue
		}

		value := 0.0
		if rawValue, ok := entryMap[cfg.ValuePath]; ok {
			value = toFloat64(rawValue)
		}

		labels := make([]string, len(commonLabels), len(commonLabels)+1)
		copy(labels, commonLabels)
		labels = append(labels, key)

		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
	}
}

// collectConditionsMetric collects conditions metrics
func (c *ConfigurableCollector) collectConditionsMetric(
	ch chan<- prometheus.Metric,
	desc *prometheus.Desc,
	obj *unstructured.Unstructured,
	cfg *MetricConfig,
	commonLabels []string,
) {
	// Default field names
	typeField := "type"
	statusField := "status"
	reasonField := "reason"

	if cfg.Condition != nil {
		if cfg.Condition.TypeField != "" {
			typeField = cfg.Condition.TypeField
		}

		if cfg.Condition.StatusField != "" {
			statusField = cfg.Condition.StatusField
		}

		if cfg.Condition.ReasonField != "" {
			reasonField = cfg.Condition.ReasonField
		}
	}

	conditions := extractFieldSlice(obj, cfg.Path)

	for _, condData := range conditions {
		condMap, ok := condData.(map[string]any)
		if !ok {
			continue
		}

		condType, _ := condMap[typeField].(string)
		condStatus, _ := condMap[statusField].(string)
		condReason, _ := condMap[reasonField].(string)

		if condType == "" {
			continue
		}

		labels := make([]string, len(commonLabels), len(commonLabels)+3)
		copy(labels, commonLabels)
		labels = append(labels, condType, condStatus, condReason)

		value := 0.0
		if strings.EqualFold(condStatus, "true") {
			value = 1.0
		}

		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labels...)
	}
}
