# Domain Collector

The Domain collector monitors domain health by performing DNS lookups, HTTP checks, and certificate validation.

## Configuration

### YAML Configuration

```yaml
collectors:
  domain:
    domains:
      - example.com
      - api.example.com
    checkTimeout: "5s"
    checkInterval: "5m"
    includeCertCheck: true
    includeHTTPCheck: true
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `domains` | []string | `[]` | List of domains to monitor |
| `checkTimeout` | duration | `5s` | Timeout for each health check |
| `checkInterval` | duration | `5m` | Interval between check cycles |
| `includeCertCheck` | bool | `true` | Enable TLS certificate validation |
| `includeHTTPCheck` | bool | `true` | Enable HTTP connectivity checks |

### Environment Variables

All configuration can be overridden using environment variables with the prefix `COLLECTORS_DOMAIN_`:

| Environment Variable | Maps To | Example |
|---------------------|---------|---------|
| `COLLECTORS_DOMAIN_DOMAINS` | `domains` | `example.com,api.example.com` |
| `COLLECTORS_DOMAIN_CHECK_TIMEOUT` | `checkTimeout` | `10s` |
| `COLLECTORS_DOMAIN_CHECK_INTERVAL` | `checkInterval` | `10m` |
| `COLLECTORS_DOMAIN_INCLUDE_CERT_CHECK` | `includeCertCheck` | `true` |
| `COLLECTORS_DOMAIN_INCLUDE_HTTP_CHECK` | `includeHTTPCheck` | `false` |

## Metrics

### `sealos_domain_status`

**Type:** Gauge
**Labels:**
- `domain`: Domain name being monitored
- `ip`: Resolved IP address
- `check_type`: Type of check performed (`dns`, `cert`, `http`)
- `error_type`: Error type if check failed (empty if successful)

**Values:**
- `1`: Check passed
- `0`: Check failed

**Example:**
```promql
sealos_domain_status{domain="example.com",ip="93.184.216.34",check_type="dns",error_type=""} 1
sealos_domain_status{domain="example.com",ip="93.184.216.34",check_type="http",error_type=""} 1
sealos_domain_status{domain="example.com",ip="93.184.216.34",check_type="cert",error_type=""} 1
sealos_domain_status{domain="bad.example.com",ip="",check_type="dns",error_type="no_such_host"} 0
```

### `sealos_domain_cert_expiry_seconds`

**Type:** Gauge
**Labels:**
- `domain`: Domain name being monitored
- `ip`: IP address of the endpoint
- `error_type`: Error type if cert check failed

**Description:** Time in seconds until the TLS certificate expires. Negative values indicate expired certificates.

**Example:**
```promql
sealos_domain_cert_expiry_seconds{domain="example.com",ip="93.184.216.34",error_type=""} 7776000
sealos_domain_cert_expiry_seconds{domain="expired.example.com",ip="1.2.3.4",error_type=""} -86400
```

### `sealos_domain_response_time_seconds`

**Type:** Gauge
**Labels:**
- `domain`: Domain name being monitored
- `ip`: IP address of the endpoint

**Description:** Response time for the domain health check in seconds.

**Example:**
```promql
sealos_domain_response_time_seconds{domain="example.com",ip="93.184.216.34"} 0.125
```

## Collector Type

**Type:** Polling
**Leader Election Required:** No

The Domain collector runs independently on each node and polls configured domains at regular intervals.
