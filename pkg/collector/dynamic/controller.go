// Package dynamic provides a generic dynamic client-based collector framework for monitoring CRDs.
package dynamic

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// ControllerConfig defines the configuration for a dynamic controller
type ControllerConfig struct {
	// GVR is the GroupVersionResource to watch
	GVR schema.GroupVersionResource

	// Namespace to watch (empty string for cluster-scoped or all namespaces)
	Namespace string

	// ResyncPeriod is the resync interval for the informer
	ResyncPeriod time.Duration

	// EventHandler is the callback interface for resource events
	EventHandler EventHandler
}

// Controller is a generic dynamic client controller that watches CRDs
type Controller struct {
	config         *ControllerConfig
	dynamicClient  dynamic.Interface
	informer       cache.SharedIndexInformer
	informerStopCh chan struct{}
	logger         *log.Entry
}

// NewController creates a new dynamic controller
func NewController(
	dynamicClient dynamic.Interface,
	config *ControllerConfig,
	logger *log.Entry,
) (*Controller, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if config.EventHandler == nil {
		return nil, errors.New("event handler cannot be nil")
	}

	if dynamicClient == nil {
		return nil, errors.New("dynamic client cannot be nil")
	}

	// Set default resync period if not specified
	if config.ResyncPeriod == 0 {
		config.ResyncPeriod = 10 * time.Minute
	}

	return &Controller{
		config:        config,
		dynamicClient: dynamicClient,
		logger:        logger,
	}, nil
}

// Start starts the controller and begins watching resources
func (c *Controller) Start(ctx context.Context) error {
	c.logger.WithFields(log.Fields{
		"gvr":       c.config.GVR.String(),
		"namespace": c.config.Namespace,
	}).Info("Starting dynamic controller")

	// Create dynamic informer factory
	var factory dynamicinformer.DynamicSharedInformerFactory
	if c.config.Namespace != "" {
		factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			c.dynamicClient,
			c.config.ResyncPeriod,
			c.config.Namespace,
			nil,
		)
	} else {
		factory = dynamicinformer.NewDynamicSharedInformerFactory(
			c.dynamicClient,
			c.config.ResyncPeriod,
		)
	}

	// Get informer for the specific GVR
	c.informer = factory.ForResource(c.config.GVR).Informer()

	// Register event handlers
	_, err := c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			if u, ok := obj.(*unstructured.Unstructured); ok {
				c.config.EventHandler.OnAdd(u)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldU, oldOk := oldObj.(*unstructured.Unstructured)

			newU, newOk := newObj.(*unstructured.Unstructured)
			if oldOk && newOk {
				c.config.EventHandler.OnUpdate(oldU, newU)
			}
		},
		DeleteFunc: func(obj any) {
			// Handle DeletedFinalStateUnknown
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					c.logger.WithField("object", obj).Error("Failed to decode deleted object")
					return
				}

				u, ok = tombstone.Obj.(*unstructured.Unstructured)
				if !ok {
					c.logger.WithField("object", tombstone.Obj).
						Error("Tombstone contained object that is not Unstructured")
					return
				}
			}

			c.config.EventHandler.OnDelete(u)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	// Start informer
	c.informerStopCh = make(chan struct{})
	go c.informer.Run(c.informerStopCh)

	// Wait for cache to sync
	c.logger.Info("Waiting for informer cache to sync")

	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		close(c.informerStopCh)
		return errors.New("failed to sync informer cache")
	}

	c.logger.Info("Dynamic controller started and cache synced")

	return nil
}

// Stop stops the controller
func (c *Controller) Stop() error {
	c.logger.Info("Stopping dynamic controller")

	if c.informerStopCh != nil {
		close(c.informerStopCh)
		c.informerStopCh = nil
	}

	return nil
}

// HasSynced returns true if the informer cache has synced
func (c *Controller) HasSynced() bool {
	if c.informer == nil {
		return false
	}
	return c.informer.HasSynced()
}

// GetStore returns the informer's store
func (c *Controller) GetStore() cache.Store {
	if c.informer == nil {
		return nil
	}
	return c.informer.GetStore()
}
