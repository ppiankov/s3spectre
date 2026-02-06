# Changelog

All notable changes to S3Spectre will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.3] - 2026-02-07

### Documentation
- Add comprehensive discover mode documentation to README
- Create QUICKSTART.md guide with detailed examples
- Document all discover command flags and options
- Add 6 common use case examples
- Include IAM permissions requirements
- Add troubleshooting section

## [0.1.2] - 2026-02-06

### Added
- **Discovery Mode**: New `s3spectre discover` command for AWS-only scanning without code references
  - Discovers all S3 buckets across AWS account without requiring code scanning
  - Risk-based scoring algorithm with 7 factors (age, inactivity, empty, tags, versioning, encryption, public access)
  - Multi-region support (scans all enabled AWS regions by default)
  - Optional security checks: `--check-encryption` and `--check-public` flags
  - Configurable thresholds: `--age-threshold-days` (365) and `--inactive-days` (180)
  - CI/CD integration: `--fail-on-unused` and `--fail-on-risky` flags
  - Identifies unused, risky, inactive buckets and version sprawl
  - Full text and JSON report support

### Enhanced
- Extended `BucketInfo` type with new fields:
  - `LastActivity`, `DaysSinceActivity`, `AgeInDays` for activity tracking
  - `ObjectCount` for approximate bucket size
  - `Encryption` and `PublicAccess` for security configuration
- New analyzer statuses: `StatusRisky` and `StatusInactive`
- Enhanced inspector with `DiscoverAllBuckets()` method for code-independent scanning

### Use Cases
- Initial AWS account audits and bucket discovery
- Finding forgotten or unused S3 buckets
- Security risk assessment (public access, missing encryption)
- Version sprawl detection across entire AWS account
- Cost optimization opportunities identification

## [0.1.1] - 2026-01-26
- Fix CI failures and code formatting issues.

## [0.1.0] - 2026-01-26
- Initial MVP release.
