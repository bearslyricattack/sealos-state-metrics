package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/zijiren233/sealos-state-metric/pkg/util"
	"k8s.io/klog/v2"
)

// DomainHealth represents the health status of a domain
type DomainHealth struct {
	Domain       string
	Namespace    string
	Ingress      string

	// HTTP check
	HTTPOk       bool
	HTTPError    string
	ResponseTime time.Duration

	// DNS check
	DNSOk    bool
	DNSError string
	IPs      []string

	// Certificate check
	CertOk       bool
	CertError    string
	CertExpiry   time.Duration

	LastChecked  time.Time
}

// DomainChecker performs health checks on domains
type DomainChecker struct {
	timeout      time.Duration
	checkHTTP    bool
	checkDNS     bool
	checkCert    bool
}

// NewDomainChecker creates a new domain checker
func NewDomainChecker(timeout time.Duration, checkHTTP, checkDNS, checkCert bool) *DomainChecker {
	return &DomainChecker{
		timeout:   timeout,
		checkHTTP: checkHTTP,
		checkDNS:  checkDNS,
		checkCert: checkCert,
	}
}

// Check performs all enabled checks on a domain
func (dc *DomainChecker) Check(ctx context.Context, domain, namespace, ingress string) *DomainHealth {
	health := &DomainHealth{
		Domain:      domain,
		Namespace:   namespace,
		Ingress:     ingress,
		LastChecked: time.Now(),
	}

	// HTTP check
	if dc.checkHTTP {
		url := fmt.Sprintf("https://%s", domain)
		result := util.CheckHTTP(ctx, url, dc.timeout)
		health.HTTPOk = result.Success
		health.HTTPError = result.Error
		health.ResponseTime = result.ResponseTime

		if !result.Success {
			// Try HTTP if HTTPS fails
			url = fmt.Sprintf("http://%s", domain)
			result = util.CheckHTTP(ctx, url, dc.timeout)
			health.HTTPOk = result.Success
			health.HTTPError = result.Error
			health.ResponseTime = result.ResponseTime
		}

		klog.V(4).InfoS("HTTP check completed",
			"domain", domain,
			"success", health.HTTPOk,
			"responseTime", health.ResponseTime)
	}

	// DNS check
	if dc.checkDNS {
		result := util.CheckDNS(ctx, domain, dc.timeout)
		health.DNSOk = result.Success
		health.DNSError = result.Error
		health.IPs = result.IPs

		klog.V(4).InfoS("DNS check completed",
			"domain", domain,
			"success", health.DNSOk,
			"ips", len(health.IPs))
	}

	// Certificate check
	if dc.checkCert {
		certInfo, err := util.GetTLSCert(domain, dc.timeout)
		if err != nil {
			health.CertOk = false
			health.CertError = err.Error()
		} else {
			health.CertOk = certInfo.IsValid
			health.CertExpiry = certInfo.ExpiresIn
			if !certInfo.IsValid {
				health.CertError = "certificate expired or not yet valid"
			}
		}

		klog.V(4).InfoS("Certificate check completed",
			"domain", domain,
			"success", health.CertOk,
			"expiresIn", health.CertExpiry)
	}

	return health
}
