package cert

import (
	"fmt"

	"github.com/zijiren233/sealos-state-metric/pkg/collector"
	"github.com/zijiren233/sealos-state-metric/pkg/collector/base"
	"github.com/zijiren233/sealos-state-metric/pkg/registry"
	"k8s.io/klog/v2"
)

const collectorName = "cert"

func init() {
	registry.MustRegister(collectorName, NewCollector)
}

// NewCollector creates a new Certificate collector
func NewCollector(factoryCtx *collector.FactoryContext) (collector.Collector, error) {
	// 1. Start with hard-coded defaults
	cfg := NewDefaultConfig()

	// 2. Load configuration from ConfigLoader pipe (file -> env)
	// ConfigLoader is never nil and handles priority: defaults < file < env
	if err := factoryCtx.ConfigLoader.LoadModuleConfig("collectors.cert", cfg); err != nil {
		klog.V(4).InfoS("Failed to load cert collector config, using defaults", "error", err)
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("cert collector is not enabled")
	}

	c := &Collector{
		BaseCollector: base.NewBaseCollector(collectorName, collector.TypeInformer),
		client:        factoryCtx.Client,
		config:        cfg,
		certs:         make(map[string]*CertData),
		stopCh:        make(chan struct{}),
	}

	c.initMetrics(factoryCtx.MetricsNamespace)
	c.SetCollectFunc(c.collect)

	return c, nil
}
