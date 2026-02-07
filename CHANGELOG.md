# Changelog

All notable changes to S3Spectre will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-07

### Added

- Scan mode (`s3spectre scan`): cross-references S3 bucket references in code against live AWS state to detect drift, including missing buckets, stale prefixes, version sprawl, and lifecycle misconfigurations
- Discover mode (`s3spectre discover`): inspects all S3 buckets in an AWS account without requiring code references, with risk-based scoring for unused, inactive, and misconfigured buckets
- Multi-region support: scans all enabled AWS regions by default, with `--regions` and `--all-regions` flags for control
- Configurable concurrency for S3 API calls (`--concurrency`)
- Repository scanners for Terraform, YAML, JSON, .env files, and source code (regex-based S3 URL and bucket name extraction)
- Text and JSON output formats, JSON output compatible with SpectreHub
- CI/CD fail flags: `--fail-on-missing`, `--fail-on-stale`, `--fail-on-version-sprawl`, `--fail-on-unused`, `--fail-on-risky`
- Optional security surface checks in discover mode: `--check-encryption` and `--check-public`
- Unused bucket detection with multi-factor scoring (code references, emptiness, deprecated tags)
- Automatic retry with exponential backoff for transient S3 API errors
- Enhanced error messages with actionable suggestions for common AWS failures (credentials, permissions, rate limiting)
