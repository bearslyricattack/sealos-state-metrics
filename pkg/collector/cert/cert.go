package cert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zijiren233/sealos-state-metric/pkg/collector/base"
	"github.com/zijiren233/sealos-state-metric/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	tlsSecretType = "kubernetes.io/tls"
)

// CertData represents certificate data with metadata
type CertData struct {
	Namespace   string
	Secret      string
	CertType    string // "tls.crt" or "ca.crt"
	CertInfo    *util.CertInfo
}

// Collector collects certificate metrics
type Collector struct {
	*base.BaseCollector

	client   kubernetes.Interface
	config   *Config
	informer cache.SharedIndexInformer
	stopCh   chan struct{}

	mu    sync.RWMutex
	certs map[string]*CertData // key: namespace/secret/type

	// Metrics
	certExpiry  *prometheus.Desc
	certInfo    *prometheus.Desc
	certInvalid *prometheus.Desc
}

// initMetrics initializes Prometheus metric descriptors
func (c *Collector) initMetrics(namespace string) {
	c.certExpiry = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cert", "expiry_seconds"),
		"Certificate expiry time in seconds",
		[]string{"namespace", "secret", "cert_type", "common_name"},
		nil,
	)
	c.certInfo = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cert", "info"),
		"Certificate information",
		[]string{"namespace", "secret", "cert_type", "common_name", "issuer", "not_before", "not_after"},
		nil,
	)
	c.certInvalid = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "cert", "invalid"),
		"Invalid certificate (1=invalid, 0=valid)",
		[]string{"namespace", "secret", "cert_type", "error"},
		nil,
	)

	// Register descriptors
	c.MustRegisterDesc(c.certExpiry)
	c.MustRegisterDesc(c.certInfo)
	c.MustRegisterDesc(c.certInvalid)
}

// Start starts the collector
func (c *Collector) Start(ctx context.Context) error {
	if err := c.BaseCollector.Start(ctx); err != nil {
		return err
	}

	// Create informer factory
	factory := informers.NewSharedInformerFactory(c.client, 10*time.Minute)

	// Create secret informer
	c.informer = factory.Core().V1().Secrets().Informer()

	// Add event handlers
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			c.processSecret(secret)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			secret := newObj.(*corev1.Secret)
			c.processSecret(secret)
		},
		DeleteFunc: func(obj interface{}) {
			secret := obj.(*corev1.Secret)
			c.mu.Lock()
			defer c.mu.Unlock()

			// Delete all certs from this secret
			prefix := secret.Namespace + "/" + secret.Name + "/"
			for key := range c.certs {
				if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					delete(c.certs, key)
				}
			}
			klog.V(4).InfoS("Secret deleted, removed certs", "secret", secret.Namespace+"/"+secret.Name)
		},
	})

	// Start informer
	factory.Start(c.stopCh)

	// Wait for cache sync
	klog.InfoS("Waiting for cert informer cache sync")
	if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
		return fmt.Errorf("failed to sync cert informer cache")
	}

	klog.InfoS("Cert collector started successfully")
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

// processSecret processes a secret to extract certificate information
func (c *Collector) processSecret(secret *corev1.Secret) {
	// Only process TLS secrets
	if secret.Type != tlsSecretType {
		return
	}

	// Check namespace filter
	if len(c.config.Namespaces) > 0 {
		found := false
		for _, ns := range c.config.Namespaces {
			if ns == secret.Namespace {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Parse tls.crt
	if tlsCert, ok := secret.Data["tls.crt"]; ok {
		key := certKey(secret.Namespace, secret.Name, "tls.crt")
		certInfo := util.ParseCertificateSafe(tlsCert)
		c.certs[key] = &CertData{
			Namespace: secret.Namespace,
			Secret:    secret.Name,
			CertType:  "tls.crt",
			CertInfo:  certInfo,
		}
		klog.V(4).InfoS("Parsed TLS certificate",
			"secret", secret.Namespace+"/"+secret.Name,
			"commonName", certInfo.CommonName,
			"expiresIn", certInfo.ExpiresIn)
	}

	// Parse ca.crt if present
	if caCert, ok := secret.Data["ca.crt"]; ok {
		key := certKey(secret.Namespace, secret.Name, "ca.crt")
		certInfo := util.ParseCertificateSafe(caCert)
		c.certs[key] = &CertData{
			Namespace: secret.Namespace,
			Secret:    secret.Name,
			CertType:  "ca.crt",
			CertInfo:  certInfo,
		}
		klog.V(4).InfoS("Parsed CA certificate",
			"secret", secret.Namespace+"/"+secret.Name,
			"commonName", certInfo.CommonName,
			"expiresIn", certInfo.ExpiresIn)
	}
}

// collect collects metrics
func (c *Collector) collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, cert := range c.certs {
		if cert.CertInfo.Error != "" {
			// Invalid certificate
			ch <- prometheus.MustNewConstMetric(
				c.certInvalid,
				prometheus.GaugeValue,
				1,
				cert.Namespace,
				cert.Secret,
				cert.CertType,
				cert.CertInfo.Error,
			)
		} else {
			// Valid certificate - emit expiry metric
			ch <- prometheus.MustNewConstMetric(
				c.certExpiry,
				prometheus.GaugeValue,
				cert.CertInfo.ExpiresIn.Seconds(),
				cert.Namespace,
				cert.Secret,
				cert.CertType,
				cert.CertInfo.CommonName,
			)

			// Emit info metric
			ch <- prometheus.MustNewConstMetric(
				c.certInfo,
				prometheus.GaugeValue,
				1,
				cert.Namespace,
				cert.Secret,
				cert.CertType,
				cert.CertInfo.CommonName,
				cert.CertInfo.Issuer,
				cert.CertInfo.NotBefore.Format(time.RFC3339),
				cert.CertInfo.NotAfter.Format(time.RFC3339),
			)

			// Emit invalid metric (0 = valid)
			ch <- prometheus.MustNewConstMetric(
				c.certInvalid,
				prometheus.GaugeValue,
				0,
				cert.Namespace,
				cert.Secret,
				cert.CertType,
				"",
			)
		}
	}
}

// certKey generates a unique key for a certificate
func certKey(namespace, secret, certType string) string {
	return namespace + "/" + secret + "/" + certType
}
