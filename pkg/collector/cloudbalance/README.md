# CloudBalance Collector

The CloudBalance collector monitors cloud account balances across multiple cloud providers.

## Supported Cloud Providers

- **Alibaba Cloud** (`alicloud`)
- **Tencent Cloud** (`tencentcloud`)
- **VolcEngine** (`volcengine`)

## Configuration

### YAML Configuration

```yaml
collectors:
  cloudbalance:
    checkInterval: "5m"
    accounts:
      - provider: alicloud
        accountId: "123456"
        accessKeyId: "YOUR_ACCESS_KEY_ID"
        accessKeySecret: "YOUR_ACCESS_KEY_SECRET"
        regionId: "cn-hangzhou"
      - provider: tencentcloud
        accountId: "987654"
        accessKeyId: "YOUR_SECRET_ID"
        accessKeySecret: "YOUR_SECRET_KEY"
        regionId: "ap-guangzhou"
      - provider: volcengine
        accountId: "111222"
        accessKeyId: "YOUR_ACCESS_KEY_ID"
        accessKeySecret: "YOUR_ACCESS_KEY_SECRET"
        regionId: "cn-beijing"
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `checkInterval` | duration | `5m` | Interval between balance checks |
| `accounts` | []Account | `[]` | List of cloud accounts to monitor |

### Account Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | Yes | Cloud provider (`alicloud`, `tencentcloud`, `volcengine`) |
| `accountId` | string | Yes | Account identifier (for labeling) |
| `accessKeyId` | string | Yes | Cloud provider access key ID |
| `accessKeySecret` | string | Yes | Cloud provider access key secret |
| `regionId` | string | No | Cloud provider region |

### Environment Variables

| Environment Variable | Maps To | Example |
|---------------------|---------|---------|
| `COLLECTORS_CLOUDBALANCE_CHECK_INTERVAL` | `checkInterval` | `10m` |

**Note:** Account credentials should be configured via Kubernetes Secrets or a secure configuration file, not environment variables.

## Metrics

### `sealos_cloudbalance_balance`

**Type:** Gauge
**Labels:**
- `provider`: Cloud provider name (`alicloud`, `tencentcloud`, `volcengine`)
- `account_id`: Account identifier from configuration

**Description:** Current account balance in the cloud provider's base currency (usually CNY/USD). Negative values indicate debt.

**Example:**
```promql
sealos_cloudbalance_balance{provider="alicloud",account_id="123456"} 1580.50
sealos_cloudbalance_balance{provider="tencentcloud",account_id="987654"} 2340.88
sealos_cloudbalance_balance{provider="volcengine",account_id="111222"} -125.30
```

## Use Cases

### Alerting on Low Balance

```promql
# Alert when balance drops below 100
sealos_cloudbalance_balance < 100

# Alert on negative balance (debt)
sealos_cloudbalance_balance < 0

# Alert when balance drops below 10% of normal (assuming 1000 is normal)
sealos_cloudbalance_balance < 100
```

### Monitoring Balance Trends

```promql
# Balance decrease rate (per day)
(sealos_cloudbalance_balance - sealos_cloudbalance_balance offset 1d) / 1

# Predict when balance will run out (linear extrapolation)
predict_linear(sealos_cloudbalance_balance[1w], 86400 * 7)

# Total balance across all accounts
sum(sealos_cloudbalance_balance)

# Accounts in debt
count(sealos_cloudbalance_balance < 0)
```

## Security Considerations

### Credential Management

**DO NOT** store credentials in plain text configuration files or environment variables in production. Use one of these secure methods:

#### 1. Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-credentials
type: Opaque
stringData:
  accounts.yaml: |
    collectors:
      cloudbalance:
        accounts:
          - provider: alicloud
            accountId: "123456"
            accessKeyId: "YOUR_KEY"
            accessKeySecret: "YOUR_SECRET"
            regionId: "cn-hangzhou"
```

Mount this secret as a file and pass the file path to the application.

#### 2. External Secret Managers

Use tools like:
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Kubernetes External Secrets Operator

#### 3. Cloud Provider IAM Roles

When running on the same cloud provider, use IAM roles/service accounts instead of explicit credentials.

## API Permissions Required

### Alibaba Cloud

Required permission: `bss:QueryAccountBalance`

### Tencent Cloud

Required permission: `billing:DescribeAccountBalance`

### VolcEngine

Required permission: `billing:QueryBalanceAcct`

## Collector Type

**Type:** Polling
**Leader Election Required:** Yes

The CloudBalance collector polls cloud provider APIs at regular intervals. Leader election ensures only one instance makes API calls, avoiding duplicate requests and staying within rate limits.

## Rate Limiting

Cloud providers may impose rate limits on billing API calls. The default 5-minute check interval is designed to stay well within typical rate limits. If you monitor many accounts, consider increasing the interval.
