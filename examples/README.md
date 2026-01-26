# S3Spectre Examples

This directory contains example files to test S3Spectre.

## Sample Repository

The `sample-repo/` directory contains a mock repository with various S3 references:

- **config.yaml**: YAML configuration with bucket names
- **app.py**: Python application with S3 SDK usage
- **infrastructure.tf**: Terraform S3 bucket definitions
- **.env.example**: Environment variables with bucket names

## Running S3Spectre on Examples

```bash
# Scan the sample repository
s3spectre scan --repo ./examples/sample-repo

# Generate JSON report
s3spectre scan --repo ./examples/sample-repo --format json --output example-report.json
```

## Expected References

The sample repository contains these S3 references:

1. `my-app-data-prod` - Primary application data bucket
2. `my-app-backups` - Backup bucket
3. `app-logs-prod` - Logging bucket
4. `user-uploads-prod` - User uploads bucket with prefixes:
   - `/media/`
   - `/thumbnails/`

## Testing Without AWS

S3Spectre will scan the repository and find references even without AWS credentials. However, to perform drift analysis, you'll need:

1. Valid AWS credentials configured
2. Appropriate S3 permissions
3. Actual S3 buckets in your account

For testing purposes, you can create test buckets:

```bash
# Using AWS CLI
aws s3 mb s3://my-app-data-prod
aws s3 mb s3://my-app-backups
aws s3 mb s3://app-logs-prod
aws s3 mb s3://user-uploads-prod

# Add some test objects
echo "test" | aws s3 cp - s3://user-uploads-prod/media/test.txt
echo "test" | aws s3 cp - s3://user-uploads-prod/thumbnails/test.jpg
```

Then run S3Spectre to see the analysis.
