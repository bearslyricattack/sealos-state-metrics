package namespace

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/labring/sealos-state-metrics/pkg/collector"
	"github.com/labring/sealos-state-metrics/pkg/collector/base"
	"github.com/labring/sealos-state-metrics/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const collectorName = "namespace"

func init() {
	registry.MustRegister(collectorName, NewCollector)
}

// NewCollector creates a new Namespace collector.
func NewCollector(factoryCtx *collector.FactoryContext) (collector.Collector, error) {
	client, err := factoryCtx.GetClient()
	if err != nil {
		return nil, fmt.Errorf("kubernetes client is required but not available: %w", err)
	}

	cfg := NewDefaultConfig()
	if err := factoryCtx.ConfigLoader.LoadModuleConfig("collectors.namespace", cfg); err != nil {
		factoryCtx.Logger.WithError(err).
			Debug("Failed to load namespace collector config, using defaults")
	}

	cfg.RequiredLabels = normalizeLabels(cfg.RequiredLabels)
	if err := validateRequiredLabels(cfg.RequiredLabels); err != nil {
		return nil, err
	}

	c := &Collector{
		BaseCollector: base.NewBaseCollector(
			collectorName,
			factoryCtx.Logger,
			base.WithWaitReadyOnCollect(true),
		),
		client:     client,
		config:     cfg,
		namespaces: make(map[string]*corev1.Namespace),
		stopCh:     make(chan struct{}),
		logger:     factoryCtx.Logger,
	}

	c.initMetrics(factoryCtx.MetricsNamespace)

	c.SetLifecycle(base.LifecycleFuncs{
		StartFunc: func(ctx context.Context) error {
			// Recreate stopCh to support a collector restart.
			c.stopCh = make(chan struct{})
			c.resetNamespaces()

			factory := informers.NewSharedInformerFactory(c.client, factoryCtx.InformerResyncPeriod)
			c.informer = factory.Core().V1().Namespaces().Informer()

			// Keep only fields required by the metrics. This avoids retaining the
			// full Namespace object in the process-wide informer cache.
			_ = c.informer.SetTransform(func(obj any) (any, error) {
				namespace, ok := obj.(*corev1.Namespace)
				if !ok {
					return obj, nil
				}

				labels := make(map[string]string, len(namespace.Labels))
				maps.Copy(labels, namespace.Labels)

				return &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:              namespace.Name,
						UID:               namespace.UID,
						CreationTimestamp: namespace.CreationTimestamp,
						Labels:            labels,
					},
				}, nil
			})

			_, _ = c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc:    c.handleNamespaceAdd,
				UpdateFunc: c.handleNamespaceUpdate,
				DeleteFunc: c.handleNamespaceDelete,
			})

			factory.Start(c.stopCh)
			c.logger.Info("Waiting for namespace informer cache sync")

			if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
				return errors.New("failed to sync namespace informer cache")
			}

			c.logger.Info("Namespace collector started successfully")
			c.SetReady()

			return nil
		},
		StopFunc: func() error {
			close(c.stopCh)
			return nil
		},
		CollectFunc: c.collect,
	})

	return c, nil
}

func normalizeLabels(labels []string) []string {
	seen := make(map[string]struct{}, len(labels))
	result := make([]string, 0, len(labels))

	for _, label := range labels {
		label = strings.TrimSpace(label)

		if label == "" {
			continue
		}

		if _, exists := seen[label]; exists {
			continue
		}

		seen[label] = struct{}{}
		result = append(result, label)
	}

	return result
}

func validateRequiredLabels(labels []string) error {
	for _, label := range labels {
		if problems := validation.IsQualifiedName(label); len(problems) > 0 {
			return fmt.Errorf("invalid required namespace label %q: %s", label, strings.Join(problems, "; "))
		}
	}

	return nil
}

func (c *Collector) resetNamespaces() {
	c.mu.Lock()
	c.namespaces = make(map[string]*corev1.Namespace)
	c.mu.Unlock()
}

func (c *Collector) handleNamespaceAdd(obj any) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		c.logger.WithField("object", obj).Error("Failed to cast object to Namespace")
		return
	}

	c.storeNamespace(namespace)
	c.logger.WithField("namespace", namespace.Name).Debug("Namespace added")
}

func (c *Collector) handleNamespaceUpdate(_, newObj any) {
	namespace, ok := newObj.(*corev1.Namespace)
	if !ok {
		c.logger.WithField("object", newObj).Error("Failed to cast object to Namespace")
		return
	}

	c.storeNamespace(namespace)
	c.logger.WithField("namespace", namespace.Name).Debug("Namespace updated")
}

func (c *Collector) storeNamespace(namespace *corev1.Namespace) {
	if namespace == nil {
		return
	}

	c.mu.Lock()
	c.namespaces[namespace.Name] = namespace.DeepCopy()
	c.mu.Unlock()
}

func (c *Collector) handleNamespaceDelete(obj any) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		tombstone, tombstoneOK := obj.(cache.DeletedFinalStateUnknown)
		if !tombstoneOK {
			c.logger.WithField("object", obj).Error("Failed to decode deleted namespace")
			return
		}

		namespace, ok = tombstone.Obj.(*corev1.Namespace)

		if !ok {
			c.logger.WithField("object", tombstone.Obj).
				Error("Tombstone contained object that is not a Namespace")
			return
		}
	}

	if namespace == nil {
		return
	}

	c.mu.Lock()
	delete(c.namespaces, namespace.Name)
	c.mu.Unlock()
	c.logger.WithField("namespace", namespace.Name).Debug("Namespace deleted")
}
