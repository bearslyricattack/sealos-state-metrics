//nolint:testpackage // Tests need access to private functions
package dynamic

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConfigurableCollector_HandleAdd(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name":      "metadata.name",
			"namespace": "metadata.namespace",
		},
		Metrics: []MetricConfig{
			{
				Type: "info",
				Name: "phase",
				Labels: map[string]string{
					"phase": "status.phase",
				},
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Create test object
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]any{
				"phase": "Running",
			},
		},
	}

	// Handle add
	collector.handleAdd(obj)

	// Verify resource was added
	collector.mu.RLock()
	defer collector.mu.RUnlock()

	if len(collector.resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(collector.resources))
	}

	key := "default/test-resource"
	if _, exists := collector.resources[key]; !exists {
		t.Errorf("Resource with key %q not found", key)
	}
}

func TestConfigurableCollector_HandleUpdate(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type: "gauge",
				Name: "replicas",
				Path: "spec.replicas",
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add initial object
	oldObj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "test-resource",
				"namespace": "default",
			},
			"spec": map[string]any{
				"replicas": int64(3),
			},
		},
	}
	collector.handleAdd(oldObj)

	// Update object
	newObj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "test-resource",
				"namespace": "default",
			},
			"spec": map[string]any{
				"replicas": int64(5),
			},
		},
	}
	collector.handleUpdate(oldObj, newObj)

	// Verify resource was updated
	collector.mu.RLock()
	defer collector.mu.RUnlock()

	key := "default/test-resource"

	resource, exists := collector.resources[key]
	if !exists {
		t.Fatalf("Resource with key %q not found", key)
	}

	replicas, _, _ := unstructured.NestedInt64(resource.Object, "spec", "replicas")
	if replicas != 5 {
		t.Errorf("Expected replicas=5, got %d", replicas)
	}
}

func TestConfigurableCollector_HandleDelete(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		Metrics: []MetricConfig{
			{
				Type: "gauge",
				Name: "test",
				Path: "status.value",
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add object
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "test-resource",
				"namespace": "default",
			},
		},
	}
	collector.handleAdd(obj)

	// Delete object
	collector.handleDelete(obj)

	// Verify resource was deleted
	collector.mu.RLock()
	defer collector.mu.RUnlock()

	if len(collector.resources) != 0 {
		t.Errorf("Expected 0 resources, got %d", len(collector.resources))
	}
}

func TestConfigurableCollector_CollectInfoWithPhaseLabel(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type: "info",
				Name: "phase",
				Help: "Resource phase",
				Labels: map[string]string{
					"phase": "status.phase",
				},
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add test resource
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]any{
				"phase": "Running",
			},
		},
	}
	collector.handleAdd(obj)

	// Collect metrics
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.collect(ch)
		close(ch)
	}()

	// Verify metric is emitted with phase label
	metricCount := 0
	for metric := range ch {
		metricCount++

		var m dto.Metric
		if err := metric.Write(&m); err != nil {
			t.Fatalf("Failed to write metric: %v", err)
		}

		// Extract phase label
		var phase string
		for _, label := range m.GetLabel() {
			if label.GetName() == "phase" {
				phase = label.GetValue()
				break
			}
		}

		// Verify phase label is emitted with value=1
		if phase != "Running" {
			t.Errorf("Expected 'Running' phase, got %q", phase)
		}

		if m.GetGauge().GetValue() != 1.0 {
			t.Errorf("Expected value 1.0 for info metric, got %v", m.GetGauge().GetValue())
		}
	}

	// Verify only one metric was emitted
	if metricCount != 1 {
		t.Errorf("Expected 1 metric, got %d", metricCount)
	}
}

func TestConfigurableCollector_CollectGaugeMetric(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type: "gauge",
				Name: "replicas",
				Help: "Number of replicas",
				Path: "spec.replicas",
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add test resource
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"spec": map[string]any{
				"replicas": int64(5),
			},
		},
	}
	collector.handleAdd(obj)

	// Collect metrics
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.collect(ch)
		close(ch)
	}()

	// Verify metric
	metric := <-ch

	var m dto.Metric
	if err := metric.Write(&m); err != nil {
		t.Fatalf("Failed to write metric: %v", err)
	}

	if m.GetGauge().GetValue() != 5.0 {
		t.Errorf("Expected gauge value 5.0, got %v", m.GetGauge().GetValue())
	}
}

func TestConfigurableCollector_CollectInfoMetric(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type: "info",
				Name: "info",
				Help: "Resource information",
				Labels: map[string]string{
					"version": "spec.version",
					"type":    "spec.type",
				},
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add test resource
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"spec": map[string]any{
				"version": "v1.0.0",
				"type":    "webapp",
			},
		},
	}
	collector.handleAdd(obj)

	// Collect metrics
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.collect(ch)
		close(ch)
	}()

	// Verify metric
	metric := <-ch

	var m dto.Metric
	if err := metric.Write(&m); err != nil {
		t.Fatalf("Failed to write metric: %v", err)
	}

	// Info metrics should always have value 1.0
	if m.GetGauge().GetValue() != 1.0 {
		t.Errorf("Expected info metric value 1.0, got %v", m.GetGauge().GetValue())
	}

	// Verify labels
	labels := make(map[string]string)
	for _, label := range m.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}

	if labels["version"] != "v1.0.0" {
		t.Errorf("Expected version label 'v1.0.0', got %q", labels["version"])
	}

	if labels["type"] != "webapp" {
		t.Errorf("Expected type label 'webapp', got %q", labels["type"])
	}
}

func TestConfigurableCollector_CollectConditionsMetric(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type: "conditions",
				Name: "condition",
				Help: "Resource conditions",
				Path: "status.conditions",
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add test resource with conditions
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
						"reason": "AllComponentsReady",
					},
					map[string]any{
						"type":   "Progressing",
						"status": "False",
						"reason": "Complete",
					},
				},
			},
		},
	}
	collector.handleAdd(obj)

	// Collect metrics
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.collect(ch)
		close(ch)
	}()

	// Verify metrics
	metricsFound := make(map[string]float64)
	for metric := range ch {
		var m dto.Metric
		if err := metric.Write(&m); err != nil {
			t.Fatalf("Failed to write metric: %v", err)
		}

		// Extract condition type
		var condType string
		for _, label := range m.GetLabel() {
			if label.GetName() == "type" {
				condType = label.GetValue()
				break
			}
		}

		metricsFound[condType] = m.GetGauge().GetValue()
	}

	// Verify conditions
	if metricsFound["Ready"] != 1.0 {
		t.Errorf("Ready condition: expected 1.0, got %v", metricsFound["Ready"])
	}

	if metricsFound["Progressing"] != 0.0 {
		t.Errorf("Progressing condition: expected 0.0, got %v", metricsFound["Progressing"])
	}
}

func TestConfigurableCollector_CollectCountMetric(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type:       "count",
				Name:       "phase_count",
				Help:       "Count of resources by phase",
				Path:       "status.phase",
				ValueLabel: "phase",
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add multiple test resources with different states
	resources := []struct {
		name  string
		phase string
	}{
		{"resource-1", "Running"},
		{"resource-2", "Running"},
		{"resource-3", "Running"},
		{"resource-4", "Pending"},
		{"resource-5", "Pending"},
		{"resource-6", "Failed"},
	}

	for _, r := range resources {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": r.name,
				},
				"status": map[string]any{
					"phase": r.phase,
				},
			},
		}
		collector.handleAdd(obj)
	}

	// Collect metrics
	ch := make(chan prometheus.Metric, 10)
	go func() {
		collector.collect(ch)
		close(ch)
	}()

	// Verify phase counts
	phaseCounts := make(map[string]float64)
	for metric := range ch {
		var m dto.Metric
		if err := metric.Write(&m); err != nil {
			t.Fatalf("Failed to write metric: %v", err)
		}

		// Extract phase label
		var phase string
		for _, label := range m.GetLabel() {
			if label.GetName() == "phase" {
				phase = label.GetValue()
				break
			}
		}

		phaseCounts[phase] = m.GetGauge().GetValue()
	}

	// Verify counts
	expectedCounts := map[string]float64{
		"Running": 3.0,
		"Pending": 2.0,
		"Failed":  1.0,
	}

	for phase, expectedCount := range expectedCounts {
		actualCount, found := phaseCounts[phase]
		if !found {
			t.Errorf("Phase %q not found in metrics", phase)
			continue
		}

		if actualCount != expectedCount {
			t.Errorf("Phase %q: expected count %v, got %v", phase, expectedCount, actualCount)
		}
	}

	// Verify no extra phases
	if len(phaseCounts) != len(expectedCounts) {
		t.Errorf("Expected %d phases, got %d", len(expectedCounts), len(phaseCounts))
	}
}

func TestConfigurableCollector_CollectMapStateMetric(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type:      "map_state",
				Name:      "component_phase",
				Help:      "Component phase",
				Path:      "status.components",
				ValuePath: "phase",
				KeyLabel:  "component",
			},
		},
	}

	collector := NewConfigurableCollector(crdConfig, "test", logger)

	// Add test resource with components
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "test-resource",
			},
			"status": map[string]any{
				"components": map[string]any{
					"web": map[string]any{
						"phase": "Running",
					},
					"db": map[string]any{
						"phase": "Ready",
					},
				},
			},
		},
	}
	collector.handleAdd(obj)

	// Collect metrics
	ch := make(chan prometheus.Metric, 20)
	go func() {
		collector.collect(ch)
		close(ch)
	}()

	// Verify only current states are emitted
	componentMetrics := make(map[string]string)

	metricCount := 0
	for metric := range ch {
		metricCount++

		var m dto.Metric
		if err := metric.Write(&m); err != nil {
			t.Fatalf("Failed to write metric: %v", err)
		}

		var component, state string
		for _, label := range m.GetLabel() {
			switch label.GetName() {
			case "component":
				component = label.GetValue()
			case "state":
				state = label.GetValue()
			}
		}

		componentMetrics[component] = state

		// Verify value is always 1.0 for current state
		if m.GetGauge().GetValue() != 1.0 {
			t.Errorf("Expected value 1.0 for current state, got %v", m.GetGauge().GetValue())
		}
	}

	// Verify only current states for each component (2 metrics total)
	if metricCount != 2 {
		t.Errorf("Expected 2 metrics (one per component), got %d", metricCount)
	}

	// Verify each component has its current state
	if componentMetrics["web"] != "Running" {
		t.Errorf("Component 'web': expected state 'Running', got %q", componentMetrics["web"])
	}

	if componentMetrics["db"] != "Ready" {
		t.Errorf("Component 'db': expected state 'Ready', got %q", componentMetrics["db"])
	}
}

func TestConfigurableCollector_MetricPrefixHandling(t *testing.T) {
	logger := log.NewEntry(log.StandardLogger())
	crdConfig := &CRDConfig{
		Name: "test-crd",
		CommonLabels: map[string]string{
			"name": "metadata.name",
		},
		Metrics: []MetricConfig{
			{
				Type: "gauge",
				Name: "value",
				Path: "spec.value",
				Help: "Test metric",
			},
		},
	}

	tests := []struct {
		name           string
		metricPrefix   string
		expectedPrefix string
	}{
		{
			name:           "with prefix",
			metricPrefix:   "sealos",
			expectedPrefix: "sealos_test_crd_value",
		},
		{
			name:           "empty prefix",
			metricPrefix:   "",
			expectedPrefix: "test_crd_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewConfigurableCollector(crdConfig, tt.metricPrefix, logger)

			// Get metric descriptors
			descs := collector.GetMetricDescriptors()
			if len(descs) != 1 {
				t.Fatalf("Expected 1 descriptor, got %d", len(descs))
			}

			// Get metric name from descriptor
			metricName := descs[0].String()
			if !contains(metricName, tt.expectedPrefix) {
				t.Errorf(
					"Expected metric name to contain %q, got: %s",
					tt.expectedPrefix,
					metricName,
				)
			}

			// Verify it doesn't start with underscore
			if tt.metricPrefix == "" && contains(metricName, "\"_test_crd_value\"") {
				t.Errorf(
					"Metric name should not start with underscore when prefix is empty, got: %s",
					metricName,
				)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
