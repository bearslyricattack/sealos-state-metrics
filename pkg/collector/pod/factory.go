package pod

import (
	"fmt"

	"github.com/zijiren233/sealos-state-metric/pkg/collector"
	"github.com/zijiren233/sealos-state-metric/pkg/collector/base"
	"github.com/zijiren233/sealos-state-metric/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const collectorName = "pod"

func init() {
	registry.MustRegister(collectorName, NewCollector)
}

// NewCollector creates a new Pod collector
func NewCollector(factoryCtx *collector.FactoryContext) (collector.Collector, error) {
	// 1. Start with hard-coded defaults
	cfg := NewDefaultConfig()

	// 2. Load configuration from ConfigLoader pipe (file -> env)
	// ConfigLoader is never nil and handles priority: defaults < file < env
	if err := factoryCtx.ConfigLoader.LoadModuleConfig("collectors.pod", cfg); err != nil {
		klog.V(4).InfoS("Failed to load pod collector config, using defaults", "error", err)
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("pod collector is not enabled")
	}

	c := &Collector{
		BaseCollector: base.NewBaseCollector(collectorName, collector.TypeInformer),
		client:        factoryCtx.Client,
		config:        cfg,
		pods:          make(map[string]*corev1.Pod),
		stopCh:        make(chan struct{}),
	}

	// Initialize aggregator if enabled
	if cfg.Aggregator.Enabled {
		c.aggregator = NewPodAggregator(cfg.Aggregator.WindowSize)
	}

	c.initMetrics(factoryCtx.MetricsNamespace)
	c.SetCollectFunc(c.collect)

	return c, nil
}
