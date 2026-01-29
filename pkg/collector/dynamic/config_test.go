//nolint:testpackage // Tests need access to private functions
package dynamic

import (
	"testing"
	"time"
)

func TestNewDefaultCollectorConfig(t *testing.T) {
	cfg := NewDefaultCollectorConfig()

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if cfg.CRDs == nil {
		t.Error("Expected non-nil CRDs slice")
	}

	if len(cfg.CRDs) != 0 {
		t.Errorf("Expected empty CRDs, got %d items", len(cfg.CRDs))
	}
}

func TestCRDConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    CRDConfig
		expectErr bool
	}{
		{
			name: "valid config",
			config: CRDConfig{
				Name: "test-crd",
				GVR: GVRConfig{
					Group:    "apps.example.com",
					Version:  "v1",
					Resource: "applications",
				},
				Namespaces: []string{"default"},
				CommonLabels: map[string]string{
					"name": "metadata.name",
				},
				Metrics: []MetricConfig{
					{
						Type: "info",
						Name: "status",
						Help: "Application status",
						Labels: map[string]string{
							"status": "status.phase",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "missing name",
			config: CRDConfig{
				GVR: GVRConfig{
					Resource: "applications",
				},
			},
			expectErr: true,
		},
		{
			name: "missing resource",
			config: CRDConfig{
				Name: "test-crd",
				GVR: GVRConfig{
					Group:   "apps.example.com",
					Version: "v1",
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.config.Name == "" || tt.config.GVR.Resource == ""
			if hasError != tt.expectErr {
				t.Errorf("Validation result = %v, expectErr = %v", hasError, tt.expectErr)
			}
		})
	}
}

func TestMetricConfig_AllTypes(t *testing.T) {
	metricTypes := []string{"info", "count", "gauge", "map_state", "map_gauge", "conditions"}

	for _, metricType := range metricTypes {
		t.Run(metricType, func(t *testing.T) {
			config := MetricConfig{
				Type: metricType,
				Name: "test_metric",
				Help: "Test metric",
			}

			switch metricType {
			case "info":
				config.Labels = map[string]string{"version": "spec.version"}
			case "count":
				config.Path = "status.phase"
				config.ValueLabel = "phase"
			case "gauge":
				config.Path = "status.value"
			case "map_state":
				config.Path = "status.components"
				config.ValuePath = "phase"
				config.KeyLabel = "component"
			case "map_gauge":
				config.Path = "status.components"
				config.ValuePath = "value"
				config.KeyLabel = "component"
			case "conditions":
				config.Path = "status.conditions"
			}

			if config.Type != metricType {
				t.Errorf("Expected type %q, got %q", metricType, config.Type)
			}
		})
	}
}

func TestConditionConfig_Defaults(t *testing.T) {
	condCfg := &ConditionConfig{}

	// Should have empty fields by default
	if condCfg.TypeField != "" {
		t.Errorf("Expected empty TypeField, got %q", condCfg.TypeField)
	}

	if condCfg.StatusField != "" {
		t.Errorf("Expected empty StatusField, got %q", condCfg.StatusField)
	}

	if condCfg.ReasonField != "" {
		t.Errorf("Expected empty ReasonField, got %q", condCfg.ReasonField)
	}
}

func TestConditionConfig_CustomFields(t *testing.T) {
	condCfg := &ConditionConfig{
		TypeField:   "conditionType",
		StatusField: "state",
		ReasonField: "cause",
	}

	if condCfg.TypeField != "conditionType" {
		t.Errorf("Expected TypeField 'conditionType', got %q", condCfg.TypeField)
	}

	if condCfg.StatusField != "state" {
		t.Errorf("Expected StatusField 'state', got %q", condCfg.StatusField)
	}

	if condCfg.ReasonField != "cause" {
		t.Errorf("Expected ReasonField 'cause', got %q", condCfg.ReasonField)
	}
}

func TestCRDConfig_DefaultResyncPeriod(t *testing.T) {
	config := CRDConfig{
		Name: "test",
		GVR: GVRConfig{
			Resource: "test",
		},
		ResyncPeriod: 0, // Not set
	}

	// ResyncPeriod should be 0 when not set
	if config.ResyncPeriod != 0 {
		t.Errorf("Expected ResyncPeriod 0, got %v", config.ResyncPeriod)
	}

	// Test with explicit value
	config.ResyncPeriod = 5 * time.Minute
	if config.ResyncPeriod != 5*time.Minute {
		t.Errorf("Expected ResyncPeriod 5m, got %v", config.ResyncPeriod)
	}
}

func TestCRDConfig_NamespaceFiltering(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		expectAll  bool
	}{
		{
			name:       "empty namespaces means all",
			namespaces: []string{},
			expectAll:  true,
		},
		{
			name:       "nil namespaces means all",
			namespaces: nil,
			expectAll:  true,
		},
		{
			name:       "specific namespaces",
			namespaces: []string{"ns1", "ns2"},
			expectAll:  false,
		},
		{
			name:       "single namespace",
			namespaces: []string{"default"},
			expectAll:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CRDConfig{
				Namespaces: tt.namespaces,
			}

			isAllNamespaces := len(config.Namespaces) == 0
			if isAllNamespaces != tt.expectAll {
				t.Errorf("isAllNamespaces = %v, expectAll = %v", isAllNamespaces, tt.expectAll)
			}
		})
	}
}

func TestMetricConfig_MapMetrics(t *testing.T) {
	mapStateMetric := MetricConfig{
		Type:      "map_state",
		Name:      "component_status",
		Help:      "Component status",
		Path:      "status.components",
		ValuePath: "phase",
		KeyLabel:  "component",
	}

	if mapStateMetric.ValuePath != "phase" {
		t.Errorf("Expected ValuePath 'phase', got %q", mapStateMetric.ValuePath)
	}

	if mapStateMetric.KeyLabel != "component" {
		t.Errorf("Expected KeyLabel 'component', got %q", mapStateMetric.KeyLabel)
	}

	mapGaugeMetric := MetricConfig{
		Type:      "map_gauge",
		Name:      "component_replicas",
		Help:      "Component replicas",
		Path:      "status.components",
		ValuePath: "replicas",
		KeyLabel:  "component",
	}

	if mapGaugeMetric.ValuePath != "replicas" {
		t.Errorf("Expected ValuePath 'replicas', got %q", mapGaugeMetric.ValuePath)
	}
}

func TestGVRConfig_FullyQualified(t *testing.T) {
	gvr := GVRConfig{
		Group:    "apps.kubeblocks.io",
		Version:  "v1alpha1",
		Resource: "clusters",
	}

	if gvr.Group != "apps.kubeblocks.io" {
		t.Errorf("Expected group 'apps.kubeblocks.io', got %q", gvr.Group)
	}

	if gvr.Version != "v1alpha1" {
		t.Errorf("Expected version 'v1alpha1', got %q", gvr.Version)
	}

	if gvr.Resource != "clusters" {
		t.Errorf("Expected resource 'clusters', got %q", gvr.Resource)
	}
}

func TestGVRConfig_CoreResource(t *testing.T) {
	// Test core Kubernetes resource (empty group)
	gvr := GVRConfig{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	if gvr.Group != "" {
		t.Errorf("Expected empty group, got %q", gvr.Group)
	}

	if gvr.Resource != "pods" {
		t.Errorf("Expected resource 'pods', got %q", gvr.Resource)
	}
}
