# s3spectre

[![CI](https://github.com/ppiankov/s3spectre/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/s3spectre/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ppiankov/s3spectre)](https://goreportcard.com/report/github.com/ppiankov/s3spectre)
[![ANCC](https://img.shields.io/badge/ANCC-compliant-brightgreen)](https://ancc.dev)

**s3spectre** — S3 bucket drift and lifecycle auditor. Part of [SpectreHub](https://spectrehub.dev).

## What it is

- Scan mode cross-references S3 bucket refs in code against live AWS state
- Discover mode inspects buckets for public access, missing encryption, and lifecycle gaps
- Detects missing buckets, stale prefixes, version sprawl, and drift
- Supports baseline mode to suppress known findings on repeat runs
- Outputs text, JSON, SARIF, and SpectreHub formats

## What it is NOT

- Not multi-cloud — AWS S3 only, no GCP Cloud Storage, Azure Blob Storage, or S3-compatible alternatives (MinIO, R2, Spaces); no custom endpoint support
- Not a replacement for AWS Config Rules or GuardDuty — not real-time
- Not a data scanner — never reads object contents, only metadata
- Not a remediation tool — reports only, never modifies buckets
- Not a cost calculator by default — `--estimate-cost` gives an approximate monthly figure for version-sprawl overhead and for inactive/unused bucket storage only, everything else stays unpriced
- Not an auto-remediation tool — `--suggest-lifecycle-policy` generates a suggestion to review and apply yourself; s3spectre never calls an AWS write API

## Quick start

### Homebrew

```sh
brew tap ppiankov/tap
brew install s3spectre
```

### From source

```sh
git clone https://github.com/ppiankov/s3spectre.git
cd s3spectre
make build
```

### Windows

Download `s3spectre_<version>_windows_amd64.zip` (or `_arm64`) from the [releases page](https://github.com/ppiankov/s3spectre/releases), extract it, and add the folder containing `s3spectre.exe` to `PATH`.

Alternatively, with Go installed:

```powershell
go install github.com/ppiankov/s3spectre/cmd/s3spectre@latest
```

Requires `%GOPATH%\bin` (or `%GOBIN%`) on `PATH`.

### Usage

```sh
s3spectre discover --region us-east-1 --format json
```

## CLI commands

| Command | Description |
|---------|-------------|
| `s3spectre scan` | Cross-reference code bucket refs against live S3 state |
| `s3spectre discover` | Inspect S3 buckets for waste and misconfigurations |
| `s3spectre version` | Print version |

## SpectreHub integration

s3spectre feeds S3 bucket findings into [SpectreHub](https://spectrehub.dev) for unified visibility across your infrastructure.

```sh
spectrehub collect --tool s3spectre
```

## Safety

s3spectre operates in **read-only mode**. It inspects and reports — never modifies, deletes, or alters your buckets.

## Documentation

| Document | Contents |
|----------|----------|
| [CLI Reference](docs/cli-reference.md) | Full command reference, flags, and configuration |
| [Guides](docs/guides/README.md) | Task-oriented walkthroughs: reading risk scores, team ownership rollups, cost/cleanup workflow, scan vs. discover |

## License

MIT — see [LICENSE](LICENSE).

---

Built by [Obsta Labs](https://obstalabs.dev)
