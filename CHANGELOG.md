# Changelog

All notable changes to S3Spectre will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-07-26

### Added

- `discover --estimate-cost` now also reports a `total_estimated_cost_usd` account-level sum across all buckets, instead of requiring the operator to add up per-bucket figures manually
- `--group-by-tag <key>` flag (discover) rolls the summary up by a bucket tag value (e.g. `Team`), reporting bucket count and summed risk score per value; buckets missing the tag land under an explicit `untagged` group
- `discover --check-encryption` now distinguishes the AWS-managed default KMS key (`alias/aws/s3`) from a customer-managed key with a separate, lower-severity risk factor

## [0.4.0] - 2026-07-26

### Added

- `discover --check-public` now recognizes intentionally-public bucket naming conventions (`public`, `webview`, `-cdn`, `-landing` by default, extendable via a new `public_bucket_allowlist_patterns` config key) and scores a match at reduced severity instead of full severity; a new informational `public_buckets` inventory lists every public bucket regardless of allowlist status, so nothing is dropped from evidence
- `--estimate-cost` now also approximates the monthly USD cost of full bucket storage for buckets flagged `INACTIVE` or `UNUSED_BUCKET`, via a new `estimated_storage_cost_usd` field distinct from the existing version-sprawl-overhead-only estimate
- `--suggest-lifecycle-policy` flag (discover) generates a deterministic JSON + Terraform lifecycle-rule suggestion for version-sprawl findings; informational only, s3spectre never calls an AWS write API

## [0.3.0] - 2026-07-26

### Added

- `--risk-threshold` flag and `risk_threshold` config key to tune the discover risk-score cutoff; the inactivity signal now scales with severity (past 2x/5x the inactive-days threshold) so multi-year-stale buckets can cross the default threshold on that signal alone
- Informational "Versioned Buckets" inventory (scan and discover) listing every bucket with versioning enabled, independent of the version-sprawl misconfiguration check
- `--estimate-cost` flag (discover) for an approximate monthly USD estimate of version-sprawl storage overhead, using an embedded S3 Standard pricing table
- Markdown output format (`--format markdown`) for scan and discover
- Windows prebuilt release `.zip` documented as the primary Windows install path, verified against real release assets

### Fixed

- `discover --check-encryption`/`--check-public` now actually populate encryption and public-access data; previously the underlying AWS API calls were never made, so both flags silently produced zero findings regardless of actual exposure
- `discover --regions` now actually limits which buckets are scanned; previously the flag was ignored and the whole account was always scanned
- A genuine API error while checking encryption/public-access (e.g. permission denied) no longer defaults to a false-positive "public access enabled" finding -- it's now left unknown rather than guessed
- SpectreHub links in README/QUICKSTART now point to spectrehub.dev instead of a private GitHub repo

## [0.2.2] - 2026-07-25

### Fixed

- `exclude_buckets` and `exclude_prefixes` config file keys are now applied to scan and discover output (previously parsed but ignored)
- Scan and discover now agree on which tag values mark a bucket as deprecated (previously discover recognized a `retired` tag and scan did not)
- Text report output now orders findings and risky buckets by severity/risk score instead of alphabetically, so critical findings are no longer buried among lower-severity noise
- Buckets matching known AWS-managed naming conventions (CloudTrail logs, AWS Config, ELB access logs, CloudFormation templates) no longer get "delete if not needed" advice from the generic unused-bucket heuristics; they are still flagged normally for encryption, public access, and version sprawl

### Added

- Windows quick-start instructions in README and docs/QUICKSTART.md; CI now verifies a Windows build/test leg
- Configuration file reference (`.s3spectre.yaml` keys) documented in docs/cli-reference.md

### Changed

- Consolidated duplicated config-default and region-selection logic in scan/discover into shared internal helpers

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

[Unreleased]: https://github.com/ppiankov/s3spectre/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/ppiankov/s3spectre/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/ppiankov/s3spectre/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/ppiankov/s3spectre/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/ppiankov/s3spectre/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/ppiankov/s3spectre/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/ppiankov/s3spectre/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/ppiankov/s3spectre/releases/tag/v0.1.0
