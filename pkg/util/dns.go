package util

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DNSCheckResult contains the result of a DNS check
type DNSCheckResult struct {
	Success bool
	IPs     []string
	Error   string
}

// CheckDNS performs a DNS lookup
func CheckDNS(ctx context.Context, domain string, timeout time.Duration) *DNSCheckResult {
	resolver := &net.Resolver{}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ips, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return &DNSCheckResult{
			Success: false,
			Error:   fmt.Sprintf("DNS lookup failed: %v", err),
		}
	}

	return &DNSCheckResult{
		Success: len(ips) > 0,
		IPs:     ips,
	}
}

// CheckIPReachability checks if an IP is reachable
func CheckIPReachability(ctx context.Context, ip string, port int, timeout time.Duration) bool {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}
