# S3Spectre

[![CI](https://github.com/ppiankov/s3spectre/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/s3spectre/actions/workflows/ci.yml)
[![Go 1.21+](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Static and runtime auditor for AWS S3 bucket drift, unused resources, and lifecycle misconfigurations.

Part of the [Spectre family](https://github.com/ppiankov) of infrastructure cleanup tools.

## What it is

S3Spectre correlates S3 bucket references found in your codebase against live AWS state. It operates in two modes:

- **Scan mode** cross-references code against AWS to detect drift: missing buckets, stale prefixes, version sprawl, and lifecycle gaps.
- **Discover mode** inspects your AWS account directly to find unused, unencrypted, or publicly accessible buckets without requiring any code.

Both modes produce deterministic, machine-readable output suitable for CI/CD gating.

## What it is NOT

- Not a replacement for AWS Config Rules or GuardDuty. S3Spectre does not monitor in real time.
- Not a data scanner. It never reads object contents, only metadata.
- Not a remediation tool. It reports problems and lets you decide what to do.
- Not a cost calculator. It identifies waste but does not estimate dollar amounts.
- Not a security scanner. Encryption and public access checks are surface-level flags, not a compliance audit.

## Philosophy

*Principiis obsta* -- resist the beginnings.

Infrastructure drift is not a detection problem. It is a structural problem. By the time a missing bucket breaks a deployment, the damage is done. S3Spectre is designed to surface these conditions early -- in CI, in code review, in scheduled audits -- so they can be addressed before they matter.

The tool presents evidence and lets humans decide. It does not auto-remediate, does not guess intent, and does not assign confidence scores where deterministic checks suffice.

## Quick start

```bash
git clone https://github.com/ppiankov/s3spectre.git
cd s3spectre
make build

# Scan mode: correlate code references against AWS
./bin/s3spectre scan --repo .

# Discover mode: audit all buckets in your AWS account
./bin/s3spectre discover
```

Requires valid AWS credentials (environment, profile, or IAM role).

## Usage

### Scan mode

Cross-references S3 references in code with live AWS state.

```bash
# Basic scan
s3spectre scan --repo ./my-repo

# Specific AWS profile and regions
s3spectre scan --repo . --aws-profile production --regions us-east-1,eu-west-1

# JSON output for CI/CD
s3spectre scan --repo . --format json --output report.json

# Fail the pipeline on drift
s3spectre scan --repo . --fail-on-missing --fail-on-stale --stale-days 60

# Enable unused bucket detection
s3spectre scan --repo . --check-unused --fail-on-unused

# Include file-level reference details
s3spectre scan --repo . --include-references --format json
```

**Scan flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--repo, -r` | `.` | Repository path to scan |
| `--aws-profile` | | AWS profile |
| `--aws-region` | | Single region mode |
| `--all-regions` | `true` | Scan all enabled regions |
| `--regions` | | Specific regions (comma-separated) |
| `--stale-days` | `90` | Stale prefix threshold |
| `--check-unused` | `false` | Enable unused bucket scoring |
| `--unused-threshold-days` | `180` | Unused bucket threshold |
| `--concurrency` | `10` | Max concurrent S3 API calls |
| `--format, -f` | `text` | Output format: `text` or `json` |
| `--output, -o` | stdout | Output file |
| `--fail-on-missing` | `false` | Exit non-zero on missing buckets |
| `--fail-on-stale` | `false` | Exit non-zero on stale prefixes |
| `--fail-on-version-sprawl` | `false` | Exit non-zero on version sprawl |
| `--fail-on-unused` | `false` | Exit non-zero on unused buckets |
| `--include-references` | `false` | Include reference details in output |
| `--no-progress` | `false` | Disable TTY progress indicators |

### Discover mode

Audits all S3 buckets in an AWS account without requiring code references.

```bash
# Discover all buckets across all regions
s3spectre discover

# Security surface checks
s3spectre discover --check-encryption --check-public

# Custom staleness thresholds
s3spectre discover --age-threshold-days 730 --inactive-days 365

# CI/CD gating
s3spectre discover --fail-on-unused --fail-on-risky --format json
```

**Discover flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--aws-profile` | | AWS profile |
| `--all-regions` | `true` | Scan all enabled regions |
| `--regions` | | Specific regions (comma-separated) |
| `--age-threshold-days` | `365` | Flag buckets older than N days |
| `--inactive-days` | `180` | Flag buckets inactive for N days |
| `--check-encryption` | `false` | Flag missing encryption |
| `--check-public` | `false` | Flag public access |
| `--concurrency` | `10` | Max concurrent S3 API calls |
| `--format, -f` | `text` | Output format: `text` or `json` |
| `--output, -o` | stdout | Output file |
| `--fail-on-unused` | `false` | Exit non-zero on unused buckets |
| `--fail-on-risky` | `false` | Exit non-zero on risky configs |
| `--no-progress` | `false` | Disable TTY progress indicators |

### Drift classifications

Scan mode classifies each bucket and prefix into one of:

| Status | Meaning |
|--------|---------|
| `MISSING_BUCKET` | Referenced in code, does not exist in AWS |
| `UNUSED_BUCKET` | Exists in AWS, not referenced in code |
| `MISSING_PREFIX` | Code references a prefix with no objects |
| `STALE_PREFIX` | Prefix exists but unmodified for N days |
| `VERSION_SPRAWL` | Versioning enabled, no lifecycle rules |
| `LIFECYCLE_MISCONFIG` | Many objects, no lifecycle rules |
| `OK` | Bucket and prefix match expected usage |

## Architecture

```
s3spectre/
├── cmd/s3spectre/main.go       # Entry point, delegates to commands
├── internal/
│   ├── commands/               # Cobra CLI: scan, discover, version
│   │   ├── root.go
│   │   ├── scan.go
│   │   ├── discover.go
│   │   ├── helpers.go          # Shared: error enhancement, status output
│   │   └── version.go
│   ├── scanner/                # Repository scanning (regex, YAML, Terraform, JSON, .env)
│   │   ├── scanner.go          # Orchestrator: walks files, dispatches to parsers
│   │   ├── regex.go            # S3 URL and bucket name pattern matching
│   │   ├── yaml.go
│   │   ├── terraform.go
│   │   ├── json.go
│   │   ├── env.go
│   │   └── types.go
│   ├── s3/                     # AWS S3 integration
│   │   ├── client.go           # S3 client wrapper with retry and backoff
│   │   ├── inspector.go        # Concurrent bucket and prefix inspection
│   │   └── types.go
│   ├── analyzer/               # Drift analysis and scoring
│   │   ├── analyzer.go         # Scan mode: code-vs-AWS correlation
│   │   ├── discovery.go        # Discover mode: account-wide heuristics
│   │   └── types.go
│   └── report/                 # Output generation
│       ├── text.go
│       ├── json.go
│       ├── discovery.go
│       └── types.go
├── Makefile
├── go.mod
└── go.sum
```

Key design decisions:

- `cmd/s3spectre/main.go` is minimal -- a single `Execute()` call.
- All logic lives in `internal/` to prevent external import.
- S3 API calls use a bounded worker pool (`--concurrency`) with exponential backoff.
- Scanner dispatches files to format-specific parsers based on extension.
- Analysis is deterministic: same inputs always produce the same classifications.

## Known limitations

- **No object-level scanning.** S3Spectre inspects bucket and prefix metadata. It does not list or read individual objects beyond what is needed for prefix existence and staleness checks.
- **Regex-based code scanning.** The scanner uses pattern matching, not AST parsing. It will miss dynamically constructed bucket names and may produce false positives on commented-out code.
- **No cost estimation.** The tool identifies unused resources but does not calculate storage costs.
- **IAM permissions required.** Needs `s3:ListBucket`, `s3:ListAllMyBuckets`, `s3:GetBucketLocation`, `s3:GetBucketVersioning`, `s3:GetLifecycleConfiguration`, and `s3:GetBucketTagging`. Missing permissions produce access-denied errors, not silent failures.
- **No real-time monitoring.** S3Spectre is a point-in-time scanner, not a daemon. Run it in CI or on a schedule.
- **Single AWS account.** Cross-account scanning is not supported.

## Roadmap

- Cost estimation for unused and stale resources
- Deep prefix scanning with pagination
- Replication rule validation
- IAM access path analysis
- Naming convention enforcement
- Historical trend tracking
- SpectreHub integration for cross-tool correlation

## License

MIT License -- see [LICENSE](LICENSE).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Issues and pull requests welcome.

Part of the Spectre family:
[VaultSpectre](https://github.com/ppiankov/vaultspectre) |
[ClickSpectre](https://github.com/ppiankov/clickspectre) |
[KafkaSpectre](https://github.com/ppiankov/kafkaspectre)
