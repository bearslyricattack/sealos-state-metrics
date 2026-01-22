package domain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zijiren233/sealos-state-metric/pkg/collector/base"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Collector collects domain metrics
type Collector struct {
	*base.BaseCollector

	client   kubernetes.Interface
	config   *Config
	informer cache.SharedIndexInformer
	checker  *DomainChecker
	stopCh   chan struct{}

	mu      sync.RWMutex
	domains map[string]*DomainHealth // key: namespace/ingress/domain

	// Metrics
	domainStatus       *prometheus.Desc
	domainCertExpiry   *prometheus.Desc
	domainResponseTime *prometheus.Desc
	domainIPCount      *prometheus.Desc
	domainIPTimeout    *prometheus.Desc
}

// initMetrics initializes Prometheus metric descriptors
func (c *Collector) initMetrics(namespace string) {
	c.domainStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "domain", "status"),
		"Domain status (1=ok, 0=error)",
		[]string{"namespace", "ingress", "domain", "check_type"},
		nil,
	)
	c.domainCertExpiry = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "domain", "cert_expiry_seconds"),
		"Domain certificate expiry in seconds",
		[]string{"namespace", "ingress", "domain"},
		nil,
	)
	c.domainResponseTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "domain", "response_time_seconds"),
		"Domain response time in seconds",
		[]string{"namespace", "ingress", "domain"},
		nil,
	)
	c.domainIPCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "domain", "ip_count"),
		"Number of IPs resolved for domain",
		[]string{"namespace", "ingress", "domain"},
		nil,
	)
	c.domainIPTimeout = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "domain", "ip_timeout"),
		"Whether specific IP timed out (1=timeout, 0=ok)",
		[]string{"namespace", "ingress", "domain", "ip"},
		nil,
	)

	// Register descriptors
	c.MustRegisterDesc(c.domainStatus)
	c.MustRegisterDesc(c.domainCertExpiry)
	c.MustRegisterDesc(c.domainResponseTime)
	c.MustRegisterDesc(c.domainIPCount)
	c.MustRegisterDesc(c.domainIPTimeout)
}

// Start starts the collector
func (c *Collector) Start(ctx context.Context) error {
	if err := c.BaseCollector.Start(ctx); err != nil {
		return err
	}

	// Create informer factory
	factory := informers.NewSharedInformerFactory(c.client, 10*time.Minute)

	// Create ingress informer
	c.informer = factory.Networking().V1().Ingresses().Informer()

	// Add event handlers
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			c.updateDomainList(ingress)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			ingress := newObj.(*networkingv1.Ingress)
			c.updateDomainList(ingress)
		},
		DeleteFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			c.mu.Lock()
			defer c.mu.Unlock()

			// Remove domains from this ingress
			prefix := ingress.Namespace + "/" + ingress.Name + "/"
			for key := range c.domains {
				if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					delete(c.domains, key)
				}
			}
			klog.V(4).InfoS("Ingress deleted, removed domains", "ingress", ingress.Namespace+"/"+ingress.Name)
		},
	})

	// Start informer
	factory.Start(c.stopCh)

	// Wait for cache sync
	klog.InfoS("Waiting for domain informer cache sync")
	if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
		return fmt.Errorf("failed to sync domain informer cache")
	}

	// Start polling goroutine
	go c.pollLoop()

	klog.InfoS("Domain collector started successfully")
	return nil
}

// Stop stops the collector
func (c *Collector) Stop() error {
	close(c.stopCh)
	return c.BaseCollector.Stop()
}

// HasSynced returns true if the informer has synced
func (c *Collector) HasSynced() bool {
	return c.informer != nil && c.informer.HasSynced()
}

// Interval returns the polling interval
func (c *Collector) Interval() time.Duration {
	return c.config.CheckInterval
}

// Poll performs one check cycle
func (c *Collector) Poll(ctx context.Context) error {
	c.mu.RLock()
	domains := make([]*DomainHealth, 0, len(c.domains))
	for _, d := range c.domains {
		domains = append(domains, d)
	}
	c.mu.RUnlock()

	klog.InfoS("Starting domain health checks", "count", len(domains))

	// Check domains concurrently
	var wg sync.WaitGroup
	for _, domain := range domains {
		wg.Add(1)
		go func(d *DomainHealth) {
			defer wg.Done()
			health := c.checker.Check(ctx, d.Domain, d.Namespace, d.Ingress)

			c.mu.Lock()
			key := domainKey(d.Namespace, d.Ingress, d.Domain)
			c.domains[key] = health
			c.mu.Unlock()
		}(domain)
	}

	wg.Wait()
	klog.InfoS("Domain health checks completed", "count", len(domains))

	return nil
}

// pollLoop runs the polling loop
func (c *Collector) pollLoop() {
	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	// Do initial check
	c.Poll(c.Context())

	for {
		select {
		case <-ticker.C:
			c.Poll(c.Context())
		case <-c.stopCh:
			return
		}
	}
}

// updateDomainList updates the list of domains from an ingress
func (c *Collector) updateDomainList(ingress *networkingv1.Ingress) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Extract domains from ingress rules
	for _, rule := range ingress.Spec.Rules {
		if rule.Host == "" {
			continue
		}

		key := domainKey(ingress.Namespace, ingress.Name, rule.Host)
		if _, exists := c.domains[key]; !exists {
			c.domains[key] = &DomainHealth{
				Domain:    rule.Host,
				Namespace: ingress.Namespace,
				Ingress:   ingress.Name,
			}
			klog.V(4).InfoS("Added domain for monitoring", "domain", rule.Host, "ingress", ingress.Namespace+"/"+ingress.Name)
		}
	}

	// Extract domains from TLS section
	for _, tls := range ingress.Spec.TLS {
		for _, host := range tls.Hosts {
			key := domainKey(ingress.Namespace, ingress.Name, host)
			if _, exists := c.domains[key]; !exists {
				c.domains[key] = &DomainHealth{
					Domain:    host,
					Namespace: ingress.Namespace,
					Ingress:   ingress.Name,
				}
				klog.V(4).InfoS("Added TLS domain for monitoring", "domain", host, "ingress", ingress.Namespace+"/"+ingress.Name)
			}
		}
	}
}

// collect collects metrics
func (c *Collector) collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, domain := range c.domains {
		// HTTP status
		if c.config.IncludeHTTPCheck {
			ch <- prometheus.MustNewConstMetric(
				c.domainStatus,
				prometheus.GaugeValue,
				boolToFloat64(domain.HTTPOk),
				domain.Namespace,
				domain.Ingress,
				domain.Domain,
				"http",
			)

			if domain.HTTPOk {
				ch <- prometheus.MustNewConstMetric(
					c.domainResponseTime,
					prometheus.GaugeValue,
					domain.ResponseTime.Seconds(),
					domain.Namespace,
					domain.Ingress,
					domain.Domain,
				)
			}
		}

		// DNS status
		if c.config.IncludeIPCheck {
			ch <- prometheus.MustNewConstMetric(
				c.domainStatus,
				prometheus.GaugeValue,
				boolToFloat64(domain.DNSOk),
				domain.Namespace,
				domain.Ingress,
				domain.Domain,
				"dns",
			)

			if domain.DNSOk {
				ch <- prometheus.MustNewConstMetric(
					c.domainIPCount,
					prometheus.GaugeValue,
					float64(len(domain.IPs)),
					domain.Namespace,
					domain.Ingress,
					domain.Domain,
				)
			}
		}

		// Certificate status
		if c.config.IncludeCertCheck {
			ch <- prometheus.MustNewConstMetric(
				c.domainStatus,
				prometheus.GaugeValue,
				boolToFloat64(domain.CertOk),
				domain.Namespace,
				domain.Ingress,
				domain.Domain,
				"cert",
			)

			if domain.CertOk && domain.CertExpiry > 0 {
				ch <- prometheus.MustNewConstMetric(
					c.domainCertExpiry,
					prometheus.GaugeValue,
					domain.CertExpiry.Seconds(),
					domain.Namespace,
					domain.Ingress,
					domain.Domain,
				)
			}
		}
	}
}

// domainKey generates a unique key for a domain
func domainKey(namespace, ingress, domain string) string {
	return namespace + "/" + ingress + "/" + domain
}

// boolToFloat64 converts a boolean to a float64
func boolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
