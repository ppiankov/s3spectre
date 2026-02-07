# S3Spectre Quick Start

Get started with S3Spectre in 5 minutes.

## Prerequisites

- Go 1.21 or later
- AWS credentials configured (via `~/.aws/credentials` or environment variables)
- Git repository with S3 references

## Installation

```bash
# Clone the repository
git clone https://github.com/ppiankov/s3spectre
cd s3spectre

# Build
make build

# Verify installation
./bin/s3spectre version
```

## Quick Scan

### 1. Scan Your Repository

```bash
./bin/s3spectre scan --repo /path/to/your/repo
```

This will:
1. Scan your repository for S3 bucket references
2. Query AWS S3 for bucket metadata
3. Detect drift and issues
4. Display a colored text report

### 2. Save JSON Report

```bash
./bin/s3spectre scan --repo . --format json --output s3-report.json
```

### 3. Use Specific AWS Profile

```bash
./bin/s3spectre scan --repo . --aws-profile production
```

### 4. Scan Multiple Regions

```bash
# Scan all enabled AWS regions (default)
./bin/s3spectre scan --repo . --all-regions

# Scan specific regions only
./bin/s3spectre scan --repo . --regions us-east-1,eu-west-1,ap-southeast-1
```

### 5. Detect Unused Buckets

```bash
# Enable unused bucket detection
./bin/s3spectre scan --repo . --check-unused

# Customize unused threshold
./bin/s3spectre scan --repo . --check-unused --unused-threshold-days 90
```

### 6. Fail CI on Issues

```bash
./bin/s3spectre scan --repo . --fail-on-missing
```

Returns exit code 1 if any buckets referenced in code don't exist in AWS.

## Example Output

### Text Output

```
S3Spectre Report
================

Scan Time: 2026-01-26 12:00:00
Repository: /my/repo

Summary
-------
Total Buckets Scanned: 5
OK: 3
Missing Buckets: 1
Stale Prefixes: 1

Missing Buckets
--------------------------------------------------
  [MISSING_BUCKET]: legacy-backup-bucket
    Bucket referenced in code but does not exist in AWS

Stale Prefixes
--------------------------------------------------
  [STALE_PREFIX]: prod-logs/app1/
    No modifications for 120 days (threshold: 90)

OK Buckets: 3
--------------------------------------------------
  [OK]: prod-data
  [OK]: staging-assets
  [OK]: customer-uploads
```

### JSON Output

```json
{
  "tool": "s3spectre",
  "version": "0.1.0",
  "timestamp": "2026-01-26T12:00:00Z",
  "config": {
    "repo_path": "/my/repo",
    "stale_threshold_days": 90
  },
  "summary": {
    "total_buckets": 5,
    "ok_buckets": 3,
    "missing_buckets": ["legacy-backup-bucket"],
    "stale_prefixes": ["prod-logs/app1/"]
  },
  "buckets": {
    "legacy-backup-bucket": {
      "name": "legacy-backup-bucket",
      "status": "MISSING_BUCKET",
      "message": "Bucket referenced in code but does not exist in AWS",
      "referenced_in_code": true,
      "exists_in_aws": false
    }
  }
}
```

## Common Use Cases

### 1. Pre-Deployment Check

```bash
#!/bin/bash
# In your CI/CD pipeline
s3spectre scan --repo . --fail-on-missing --format json --output s3-audit.json

if [ $? -ne 0 ]; then
  echo "S3 drift detected! Check s3-audit.json"
  exit 1
fi
```

### 2. Weekly Cleanup Report

```bash
#!/bin/bash
# Cron job to detect stale resources
s3spectre scan --repo /path/to/infra \
  --stale-days 60 \
  --format json \
  --output /reports/s3-$(date +%Y%m%d).json
```

### 3. Large S3 Estate

```bash
# Increase concurrency for faster scanning
s3spectre scan --repo . --concurrency 25
```

### 4. Multi-Region Infrastructure Audit

```bash
# Scan all regions and detect unused buckets
s3spectre scan --repo /path/to/infra \
  --all-regions \
  --check-unused \
  --format json \
  --output /reports/s3-multi-region-$(date +%Y%m%d).json
```

### 5. CI/CD Integration with All Checks

```bash
#!/bin/bash
# Comprehensive S3 audit in CI/CD
s3spectre scan --repo . \
  --all-regions \
  --check-unused \
  --fail-on-missing \
  --fail-on-stale \
  --fail-on-version-sprawl \
  --fail-on-unused \
  --format json \
  --output s3-audit.json

if [ $? -ne 0 ]; then
  echo "S3 drift detected! Check s3-audit.json"
  exit 1
fi
```

## What S3Spectre Detects

| Issue | Description | Example |
|-------|-------------|---------|
| **MISSING_BUCKET** | Bucket in code doesn't exist | `s3://old-logs` referenced but not in AWS |
| **UNUSED_BUCKET** | Bucket exists but appears unused | Not in code + empty + old tags = unused |
| **STALE_PREFIX** | No activity in N days | `s3://backups/2022/` untouched for 300 days |
| **VERSION_SPRAWL** | Versioning on, no lifecycle | Bucket has 1000s of versions piling up |
| **LIFECYCLE_MISCONFIG** | No expiration rules | Large bucket with no cleanup policy |
| **MISSING_PREFIX** | Prefix in code has no objects | Code expects `logs/app/` but it's empty |

## Next Steps

- Read the full [README.md](README.md)
- Integrate with [SpectreHub](https://github.com/ppiankov/spectrehub)
- Check out other Spectre tools:
  - [VaultSpectre](https://github.com/ppiankov/vaultspectre)
  - [KafkaSpectre](https://github.com/ppiankov/kafkaspectre)
  - [ClickSpectre](https://github.com/ppiankov/clickspectre)

## Troubleshooting

### "Permission denied" errors

Ensure your AWS credentials have these permissions:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListAllMyBuckets",
        "s3:GetBucketLocation",
        "s3:GetBucketVersioning",
        "s3:GetLifecycleConfiguration",
        "s3:GetBucketTagging",
        "s3:ListBucket",
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    }
  ]
}
```

**Note:** `ec2:DescribeRegions` is required for multi-region scanning. If you don't have this permission, use `--aws-region` to specify a single region instead.

### No references found

S3Spectre looks for:
- `s3://bucket-name/path`
- `https://bucket.s3.amazonaws.com/path`
- Terraform `aws_s3_bucket` resources
- Config fields like `bucket: my-bucket`
- Environment variables like `S3_BUCKET=my-bucket`

Make sure your repository contains these patterns.

### Slow scans

- Reduce `--stale-days` to skip deep prefix analysis
- Increase `--concurrency` (careful with AWS rate limits)
- Run against specific subdirectories instead of entire monorepo

---

Happy hunting! 👻
