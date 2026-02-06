# S3Spectre Quick Start Guide

## Installation

```bash
git clone https://github.com/ppiankov/s3spectre
cd s3spectre
make build
```

Binary will be at `./bin/s3spectre`

## Two Ways to Use S3Spectre

### 1. Discover Mode - Scan AWS Account Directly

**Use this when:**
- Doing initial AWS account audit
- Finding forgotten or unused buckets
- Identifying security risks (public access, no encryption)
- Detecting version sprawl across entire account
- No code repository available

**Basic usage:**

```bash
# Scan all buckets in all AWS regions
./bin/s3spectre discover

# Use specific AWS profile
./bin/s3spectre discover --aws-profile production

# With security checks
./bin/s3spectre discover --check-encryption --check-public
```

**Example output:**

```
S3Spectre Discovery Report
==========================

Scan Time: 2026-02-06 15:00:00
Scanning: All enabled AWS regions
Total Regions Scanned: 5

Summary
-------
Total Buckets: 42
Healthy: 30
Unused: 5
Risky: 4
Inactive: 2
Version Sprawl: 1

Unused Buckets
--------------
  [UNUSED] legacy-backups-2023 (us-east-1)
    Risk Score: 120/100
    Factors:
      - Old bucket (850 days)
      - No activity for 400 days
      - Empty bucket
      - Has deprecated tags
    Recommendations:
      - Consider archiving or deleting if not needed

Risky Buckets
-------------
  [RISKY] customer-uploads (us-west-2)
    Risk Score: 100/100
    Factors:
      - Public access enabled
      - No encryption enabled
    Recommendations:
      - Review and restrict public access if not required
      - Enable default encryption (AES256 or KMS)
```

**Common options:**

```bash
# Scan specific regions only
./bin/s3spectre discover --regions us-east-1,us-west-2

# Custom thresholds
./bin/s3spectre discover --age-threshold-days 730 --inactive-days 365

# JSON output for automation
./bin/s3spectre discover --format json --output report.json

# CI/CD mode - fail on issues
./bin/s3spectre discover --fail-on-unused --fail-on-risky
```

### 2. Scan Mode - Compare Code vs AWS

**Use this when:**
- You have a code repository that uses S3
- Finding drift between code and AWS
- Detecting missing buckets referenced in code
- Finding stale S3 paths in your codebase

**Basic usage:**

```bash
# Scan current directory
./bin/s3spectre scan --repo .

# Scan specific repository
./bin/s3spectre scan --repo /path/to/my-app

# With AWS profile
./bin/s3spectre scan --repo . --aws-profile production
```

**Example output:**

```
S3Spectre Report
================

Scan Time: 2026-02-06 15:00:00
Repository: /Users/dev/my-app
AWS Profile: production
AWS Region: multi-region (5 regions scanned)

Summary
-------
Total Buckets Scanned: 10
OK: 7
Missing Buckets: 1
Unused Buckets: 1
Stale Prefixes: 2
Version Sprawl: 1

Missing Buckets
---------------
  [MISSING_BUCKET]: legacy-data
    Bucket referenced in code but does not exist in AWS

Unused Buckets
--------------
  [UNUSED_BUCKET]: old-temp-storage
    Exists in AWS but not referenced in code
    Reasons:
      - Not referenced in code
      - Empty bucket
      - Has deprecated tags

Stale Prefixes
--------------
  [STALE_PREFIX]: backups/db/
    No modifications for 400 days

Version Sprawl
--------------
  [VERSION_SPRAWL]: assets/prod/
    Versioning enabled with no lifecycle rules
```

**Common options:**

```bash
# Check for unused buckets
./bin/s3spectre scan --repo . --check-unused

# Fail CI on specific issues
./bin/s3spectre scan --repo . --fail-on-missing --fail-on-stale

# JSON output
./bin/s3spectre scan --repo . --format json --output report.json

# Include code reference details
./bin/s3spectre scan --repo . --include-references --format json
```

## Common Use Cases

### Use Case 1: Initial AWS Account Audit

```bash
# Discover all buckets and identify risks
./bin/s3spectre discover --check-encryption --check-public --format json --output audit-2026-02-06.json
```

### Use Case 2: Find Unused Buckets for Cost Optimization

```bash
# Flag buckets unused for 6 months
./bin/s3spectre discover --inactive-days 180 --fail-on-unused
```

### Use Case 3: CI/CD Pipeline - Prevent Missing References

```bash
# In .github/workflows/ci.yml or similar
./bin/s3spectre scan --repo . --fail-on-missing --fail-on-stale --no-progress
```

### Use Case 4: Security Audit - Public Buckets

```bash
# Check for public access and encryption
./bin/s3spectre discover --check-public --check-encryption --fail-on-risky
```

### Use Case 5: Multi-Region Bucket Discovery

```bash
# Scan all regions (default behavior)
./bin/s3spectre discover

# Or specific regions
./bin/s3spectre discover --regions us-east-1,us-west-2,eu-west-1
```

### Use Case 6: Version Sprawl Detection

```bash
# Find buckets with versioning but no lifecycle rules
./bin/s3spectre discover --format json | jq '.buckets | to_entries | map(select(.value.status == "VERSION_SPRAWL"))'
```

## AWS Credentials

S3Spectre uses the standard AWS SDK credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. AWS credentials file (`~/.aws/credentials`)
3. IAM role (when running on EC2/ECS/Lambda)

**Using profiles:**

```bash
export AWS_PROFILE=production
./bin/s3spectre discover

# Or use --aws-profile flag
./bin/s3spectre discover --aws-profile production
```

## Required IAM Permissions

### For Discover Mode:

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
        "s3:GetBucketEncryption",
        "s3:GetBucketPublicAccessBlock",
        "s3:ListBucket",
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    }
  ]
}
```

### For Scan Mode:

Same as discover mode (needs AWS read access to verify bucket state).

## Tips and Best Practices

1. **Start with discover mode** on a new AWS account to get the full picture
2. **Use scan mode** for ongoing monitoring of specific codebases
3. **Run in CI/CD** with `--fail-on-missing` to catch broken references
4. **Enable security checks** (`--check-encryption`, `--check-public`) for compliance
5. **Use JSON output** for integration with other tools or dashboards
6. **Adjust concurrency** with `--concurrency` if you hit rate limits
7. **Run regularly** (weekly/monthly) to catch drift over time

## Troubleshooting

### "No credentials found"

Configure AWS credentials:
```bash
aws configure --profile production
```

### "Access Denied" errors

Check IAM permissions - you need read access to S3 and EC2 (for region discovery).

### Rate limiting

Reduce concurrency:
```bash
./bin/s3spectre discover --concurrency 5
```

### Progress indicators not showing

Progress is automatically disabled in non-TTY environments. Force disable with `--no-progress`.

## Next Steps

- See [README.md](README.md) for detailed feature documentation
- See [CHANGELOG.md](CHANGELOG.md) for version history
- See [CONTRIBUTING.md](CONTRIBUTING.md) for development guide

---

*Need help? Open an issue at https://github.com/ppiankov/s3spectre/issues*
