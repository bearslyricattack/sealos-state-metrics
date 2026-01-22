package domain

import (
	"fmt"

	"github.com/zijiren233/sealos-state-metric/pkg/collector"
	"github.com/zijiren233/sealos-state-metric/pkg/collector/base"
	"github.com/zijiren233/sealos-state-metric/pkg/registry"
	"k8s.io/klog/v2"
)

const collectorName = "domain"

func init() {
	registry.MustRegister(collectorName, NewCollector)
}

// NewCollector creates a new Domain collector
func NewCollector(factoryCtx *collector.FactoryContext) (collector.Collector, error) {
	// 1. Start with hard-coded defaults
	cfg := NewDefaultConfig()

	// 2. Load configuration from ConfigLoader pipe (file -> env)
	// ConfigLoader is never nil and handles priority: defaults < file < env
	if err := factoryCtx.ConfigLoader.LoadModuleConfig("collectors.domain", cfg); err != nil {
		klog.V(4).InfoS("Failed to load domain collector config, using defaults", "error", err)
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("domain collector is not enabled")
	}

	c := &Collector{
		BaseCollector: base.NewBaseCollector(collectorName, collector.TypePolling),
		client:        factoryCtx.Client,
		config:        cfg,
		domains:       make(map[string]*DomainHealth),
		stopCh:        make(chan struct{}),
	}

	// Create checker
	c.checker = NewDomainChecker(
		cfg.CheckTimeout,
		cfg.IncludeHTTPCheck,
		cfg.IncludeIPCheck,
		cfg.IncludeCertCheck,
	)

	c.initMetrics(factoryCtx.MetricsNamespace)
	c.SetCollectFunc(c.collect)

	return c, nil
}
