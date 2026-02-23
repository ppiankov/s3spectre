# Changelog

All notable changes to S3Spectre will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-02-23

### Added

- SpectreHub `spectre/v1` envelope output format (`--format spectrehub`) for both scan and discover modes
- `HashRegion()` function for AWS region/profile hashing

## [0.2.0] - 2026-02-22

### Added

- SARIF output format (`--format sarif`) for GitHub Security tab integration (WO-S04)
- Structured logging via `log/slog` with `--verbose` flag (WO-S02)
- Config file support: `.s3spectre.yaml` in CWD or home directory (WO-S05)
- `--timeout` flag for total operation timeout on scan and discover commands (WO-S03)
- Baseline diff mode: `--baseline` and `--update-baseline` flags for suppressing known findings (WO-S06)
- Connection banner showing AWS region and profile on scan start (WO-S11)
- "No issues detected" message when scan finds no problems (WO-S11)
- Version injection via LDFLAGS (version, commit, date in `s3spectre version` output)
- GoReleaser v2 config for multi-platform releases (WO-S07)
- Docker images via multi-stage distroless build, multi-arch manifests on ghcr.io (WO-S08)
- Homebrew formula via GoReleaser brews section (WO-S09)
- Version displayed in text report headers

### Changed

- Release workflow switched from manual builds to GoReleaser
- CI lint job now includes `go vet` step
- Makefile aligned with spectre family conventions (LDFLAGS, vet, deps, coverage targets)
- Test coverage improved: analyzer 98.8%, scanner 87.1%, report 79%, logging 100%

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

[Unreleased]: https://github.com/ppiankov/s3spectre/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/ppiankov/s3spectre/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/ppiankov/s3spectre/releases/tag/v0.1.0
