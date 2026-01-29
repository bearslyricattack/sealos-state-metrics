//nolint:testpackage // Tests need access to private function buildCollectorConfig
package kubeblocks

import (
	"testing"
	"time"
)

// TestConfig_Defaults verifies the default configuration values
func TestConfig_Defaults(t *testing.T) {
	cfg := NewDefaultConfig()

	if len(cfg.Namespaces) != 0 {
		t.Errorf("Expected empty namespaces (all), got %d", len(cfg.Namespaces))
	}

	if cfg.ResyncPeriod != 10*time.Minute {
		t.Errorf("Expected resync period 10m, got %v", cfg.ResyncPeriod)
	}
}

// TestBuildCollectorConfig verifies the generated CollectorConfig
func TestBuildCollectorConfig(t *testing.T) {
	cfg := NewDefaultConfig()
	collectorConfig := buildCollectorConfig(cfg)

	if len(collectorConfig.CRDs) != 1 {
		t.Fatalf("Expected 1 CRD, got %d", len(collectorConfig.CRDs))
	}

	crdCfg := collectorConfig.CRDs[0]

	// Verify CRD name
	if crdCfg.Name != "kubeblocks-cluster" {
		t.Errorf("Expected name 'kubeblocks-cluster', got %q", crdCfg.Name)
	}

	// Verify GVR
	if crdCfg.GVR.Group != "apps.kubeblocks.io" {
		t.Errorf("Expected group 'apps.kubeblocks.io', got %q", crdCfg.GVR.Group)
	}

	if crdCfg.GVR.Version != "v1alpha1" {
		t.Errorf("Expected version 'v1alpha1', got %q", crdCfg.GVR.Version)
	}

	if crdCfg.GVR.Resource != "clusters" {
		t.Errorf("Expected resource 'clusters', got %q", crdCfg.GVR.Resource)
	}

	// Verify common labels
	if len(crdCfg.CommonLabels) != 2 {
		t.Errorf("Expected 2 common labels, got %d", len(crdCfg.CommonLabels))
	}

	if crdCfg.CommonLabels["namespace"] != "metadata.namespace" {
		t.Errorf("Expected namespace label, got %q", crdCfg.CommonLabels["namespace"])
	}

	if crdCfg.CommonLabels["cluster"] != "metadata.name" {
		t.Errorf("Expected cluster label, got %q", crdCfg.CommonLabels["cluster"])
	}

	// Verify metrics
	if len(crdCfg.Metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(crdCfg.Metrics))
	}

	// Verify metric types
	metricTypes := make(map[string]string)
	for _, m := range crdCfg.Metrics {
		metricTypes[m.Name] = m.Type
	}

	expectedMetrics := map[string]string{
		"info":        "info",
		"phase_count": "count",
	}

	for name, expectedType := range expectedMetrics {
		actualType, found := metricTypes[name]
		if !found {
			t.Errorf("Metric %q not found", name)
			continue
		}

		if actualType != expectedType {
			t.Errorf("Metric %q: expected type %q, got %q", name, expectedType, actualType)
		}
	}
}

// TestConfig_CustomNamespaces verifies namespace configuration
func TestConfig_CustomNamespaces(t *testing.T) {
	cfg := &Config{
		Namespaces:   []string{"ns1", "ns2"},
		ResyncPeriod: 5 * time.Minute,
	}

	collectorConfig := buildCollectorConfig(cfg)
	crdCfg := collectorConfig.CRDs[0]

	if len(crdCfg.Namespaces) != 2 {
		t.Errorf("Expected 2 namespaces, got %d", len(crdCfg.Namespaces))
	}

	if crdCfg.Namespaces[0] != "ns1" || crdCfg.Namespaces[1] != "ns2" {
		t.Errorf("Expected namespaces [ns1, ns2], got %v", crdCfg.Namespaces)
	}

	if crdCfg.ResyncPeriod != 5*time.Minute {
		t.Errorf("Expected resync period 5m, got %v", crdCfg.ResyncPeriod)
	}
}
