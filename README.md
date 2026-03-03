# s3spectre

[![CI](https://github.com/ppiankov/s3spectre/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/s3spectre/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ppiankov/s3spectre)](https://goreportcard.com/report/github.com/ppiankov/s3spectre)

**s3spectre** — S3 bucket drift and lifecycle auditor. Part of [SpectreHub](https://github.com/ppiankov/spectrehub).

## What it is

- Scan mode cross-references S3 bucket refs in code against live AWS state
- Discover mode inspects buckets for public access, missing encryption, and lifecycle gaps
- Detects missing buckets, stale prefixes, version sprawl, and drift
- Supports baseline mode to suppress known findings on repeat runs
- Outputs text, JSON, SARIF, and SpectreHub formats

## What it is NOT

- Not a replacement for AWS Config Rules or GuardDuty — not real-time
- Not a data scanner — never reads object contents, only metadata
- Not a remediation tool — reports only, never modifies buckets
- Not a cost calculator — identifies waste, does not estimate dollars

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

s3spectre feeds S3 bucket findings into [SpectreHub](https://github.com/ppiankov/spectrehub) for unified visibility across your infrastructure.

```sh
spectrehub collect --tool s3spectre
```

## Safety

s3spectre operates in **read-only mode**. It inspects and reports — never modifies, deletes, or alters your buckets.

## License

MIT — see [LICENSE](LICENSE).

---

Built by [Obsta Labs](https://github.com/ppiankov)
