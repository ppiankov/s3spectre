# S3Spectre

S3Spectre is a Go-based static + runtime auditor for AWS S3 usage. It scans your application codebase for S3 bucket and object references, compares those to the actual AWS S3 state, and detects missing or unused buckets, stale prefixes, version sprawl, and lifecycle misconfigurations.

Part of the [Spectre family](https://github.com/ppiankov) of infrastructure cleanup tools.

## Why S3Spectre Exists

AWS S3 tells you what buckets and objects exist.
Your codebase tells you what buckets and paths are referenced.
Neither tells you which ones are **still actually needed**.

S3Spectre bridges that gap by correlating:
- S3 references found in code and configuration
- live inspection of AWS S3 state
- metadata such as versioning, lifecycle rules, and object age

It is designed for teams who inherit AWS accounts,
want to clean up S3 safely,
and would prefer not to discover broken references in production.

## Features (MVP)

### 1. Repository Scanner (Static S3 Reference Extraction)

Detects S3 usage across:
- **Config files**: YAML, JSON, .env
- **Infrastructure as Code**: Terraform (.tf), CloudFormation
- **Source code**: Python, Node.js, Go, Java, Shell scripts
- **S3 URLs**:
  - `s3://bucket/path/...`
  - `https://bucket.s3.amazonaws.com/...`
  - Versioned objects (`?versionId=...`)

Extracts:
- Bucket name
- Prefix/object path
- Version ID (if present)
- File + line number
- Usage context (read/write/list)

### 2. AWS S3 Inspector (Runtime Metadata)

Queries AWS via AWS SDK to collect:

**Buckets**:
- Existence
- Region
- Versioning status
- Lifecycle policies
- Creation date

**Prefix/Object metadata**:
- Existence
- Last modified timestamps
- Object count
- Total versions (if versioning enabled)
- Storage class

### 3. Analysis & Drift Engine

Matches repo references against real AWS S3 data and flags:

- ✔️ **MISSING_BUCKET**: Referenced in code but doesn't exist in AWS
- ✔️ **UNUSED_BUCKET**: Exists in AWS but not referenced in repo
- ✔️ **MISSING_PREFIX**: Repo references a prefix but no objects found
- ✔️ **STALE_PREFIX**: Prefix exists but no modifications in N days
- ✔️ **VERSION_SPRAWL**: Versioning enabled with many versions but no lifecycle rules
- ✔️ **LIFECYCLE_MISCONFIG**: Bucket has no lifecycle rules but stores many objects
- ✔️ **OK**: Bucket/prefix exists and matches expected usage

### 4. Reporting

**Text output** (human-readable with colors):
```
[MISSING_BUCKET]      s3://legacy-data
[STALE_PREFIX]        s3://backups/db/   (no updates for 400 days)
[VERSION_SPRAWL]      s3://assets/prod/  (versioning enabled, no lifecycle)
[OK]                  s3://customer-data
```

**JSON output** (CI/CD friendly):
```json
{
  "tool": "s3spectre",
  "version": "0.1.0",
  "timestamp": "2026-01-26T12:00:00Z",
  "summary": {
    "total_buckets": 10,
    "ok_buckets": 7,
    "missing_buckets": ["legacy-data"],
    "stale_prefixes": ["backups/db/"],
    "version_sprawl": ["assets/prod/"]
  }
}
```

## Installation

### From Source

```bash
git clone https://github.com/ppiankov/s3spectre
cd s3spectre
make build
```

Binary will be in `./bin/s3spectre`

### Install to GOPATH

```bash
make install
```

## Usage

### Basic Scan

```bash
s3spectre scan --repo ./my-repo
```

### Using AWS Profile

```bash
s3spectre scan --repo ./my-repo --aws-profile production
```

### Specify AWS Region

```bash
# Scan a specific region
s3spectre scan --repo ./my-repo --aws-region us-west-2

# Scan all enabled AWS regions (default)
s3spectre scan --repo ./my-repo --all-regions

# Scan specific regions only
s3spectre scan --repo ./my-repo --regions us-east-1,us-west-2,eu-west-1
```

### JSON Output

```bash
s3spectre scan --repo . --format json --output report.json
```

### Fail CI on Issues

```bash
# Fail if missing buckets found
s3spectre scan --repo . --fail-on-missing

# Fail if stale prefixes found
s3spectre scan --repo . --fail-on-stale --stale-days 60

# Fail if version sprawl detected
s3spectre scan --repo . --fail-on-version-sprawl
```

### Include Detailed References

```bash
s3spectre scan --repo . --include-references --format json
```

### Adjust Concurrency

```bash
# Use 20 concurrent API calls (default: 10)
s3spectre scan --repo . --concurrency 20
```

### Unused Bucket Detection

```bash
# Enable unused bucket detection with scoring heuristics
s3spectre scan --repo . --check-unused

# Customize unused threshold (default: 180 days)
s3spectre scan --repo . --check-unused --unused-threshold-days 90

# Fail CI if unused buckets detected
s3spectre scan --repo . --check-unused --fail-on-unused
```

### Disable Progress Indicators

```bash
# Useful for non-TTY environments or CI/CD
s3spectre scan --repo . --no-progress
```

## Configuration Options

| Flag | Description | Default |
|------|-------------|---------|
| `--repo, -r` | Path to repository to scan | `.` |
| `--aws-profile` | AWS profile to use | (default profile) |
| `--aws-region` | AWS region (single region mode) | (profile default) |
| `--all-regions` | Scan all enabled AWS regions | `true` |
| `--regions` | Specific regions to scan (comma-separated) | (all regions) |
| `--stale-days` | Days threshold for stale prefix detection | `90` |
| `--unused-threshold-days` | Days threshold for unused bucket detection | `180` |
| `--check-unused` | Enable unused bucket detection | `false` |
| `--concurrency` | Max concurrent S3 API calls | `10` |
| `--format, -f` | Output format: `text` or `json` | `text` |
| `--output, -o` | Output file (default: stdout) | stdout |
| `--fail-on-missing` | Exit with error if missing buckets found | `false` |
| `--fail-on-stale` | Exit with error if stale prefixes found | `false` |
| `--fail-on-version-sprawl` | Exit with error if version sprawl detected | `false` |
| `--fail-on-unused` | Exit with error if unused buckets found | `false` |
| `--include-references` | Include reference details in output | `false` |
| `--no-progress` | Disable progress indicators | `false` |

## Enhanced Features

### Multi-Region Support

S3Spectre can scan buckets across multiple AWS regions:

- **All regions mode** (default): Automatically scans all enabled regions in your AWS account
- **Specific regions**: Target specific regions with `--regions us-east-1,eu-west-1`
- **Single region**: Use `--aws-region` to scan a single region

The tool automatically determines each bucket's region and uses region-specific clients for accurate inspection.

### Unused Bucket Detection

Intelligent unused bucket detection using multi-factor scoring:

**Scoring factors:**
- Not referenced in code (100 points)
- Bucket is empty (50 points)
- Has deprecated tags like "deprecated", "old", "unused" (20 points)

**Default threshold:** 150 points = unused

**Use case:** Identify buckets that can be safely deleted to reduce costs and security surface area.

### Better Error Handling

Enhanced error messages with actionable suggestions:

- **Access Denied**: Suggests checking IAM permissions
- **NoCredentialProviders**: Suggests credential configuration options
- **Rate Limiting**: Suggests reducing concurrency
- **Missing paths**: Suggests verifying repository path

All S3 API calls include automatic retry logic with exponential backoff for transient errors.

### Progress Indicators

Real-time progress feedback during scans:

- Shows current operation (listing regions, inspecting buckets)
- Progress counters (e.g., "Inspecting bucket [5/10]")
- Automatically disabled in non-TTY environments (CI/CD)
- Can be disabled with `--no-progress`

## Architecture

```
s3spectre/
├── cmd/s3spectre/          # CLI entry point
├── internal/
│   ├── commands/           # Cobra CLI commands
│   ├── scanner/            # Repository scanning
│   │   ├── scanner.go      # Main scanner
│   │   ├── regex.go        # Regex-based code scanning
│   │   ├── terraform.go    # Terraform parser
│   │   ├── yaml.go         # YAML parser
│   │   ├── json.go         # JSON parser
│   │   └── env.go          # .env parser
│   ├── s3/                 # AWS S3 integration
│   │   ├── client.go       # S3 client wrapper
│   │   ├── inspector.go    # Concurrent S3 inspector
│   │   └── types.go        # S3 data types
│   ├── analyzer/           # Drift analysis
│   │   ├── analyzer.go     # Analysis logic
│   │   └── types.go        # Analysis types
│   └── report/             # Report generation
│       ├── text.go         # Text reporter
│       ├── json.go         # JSON reporter
│       └── types.go        # Report types
├── Makefile
├── go.mod
└── README.md
```

## Concurrency

S3Spectre is designed with concurrency from the ground up:

- **Parallel bucket inspection**: Multiple buckets inspected simultaneously
- **Concurrent prefix scanning**: Prefixes within buckets scanned in parallel
- **Configurable workers**: Adjust `--concurrency` based on AWS rate limits
- **Efficient for large S3 estates**: Handles hundreds of buckets efficiently

## Integration with SpectreHub

S3Spectre outputs are compatible with [SpectreHub](https://github.com/ppiankov/spectrehub), the central aggregator for all Spectre tools. Use `--format json` to generate SpectreHub-compatible reports.

Example:
```bash
s3spectre scan --repo . --format json --output .spectre/s3spectre-report.json
spectrehub collect .spectre/
```

## Development

### Build
```bash
make build
```

### Test
```bash
make test
```

### Format Code
```bash
make fmt
```

### Run Linter
```bash
make lint
```

### Clean
```bash
make clean
```

## Roadmap

Post-MVP features:
- Cost estimation for unused/stale resources
- Deep prefix scanning with pagination
- Replication rule validation
- Encryption analysis (SSE-KMS/SSE-S3)
- IAM access graphing
- Public bucket detection
- Naming convention enforcement
- Slack/Teams notifications
- Historical trend tracking

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

Areas where help is appreciated:
- Additional file format scanners (CloudFormation improvements, Pulumi, etc.)
- Enhanced S3 analysis heuristics
- Cost estimation algorithms
- Test coverage
- Documentation

## License

MIT License - see [LICENSE](LICENSE)

## Related projects

Part of the Spectre family:
- [VaultSpectre](https://github.com/ppiankov/vaultspectre) - HashiCorp Vault auditor
- [ClickSpectre](https://github.com/ppiankov/clickspectre) - ClickHouse auditor
- [KafkaSpectre](https://github.com/ppiankov/kafkaspectre) - Kafka topic auditor

## Documentation

- [Quickstart](docs/QUICKSTART.md)
- [Project summary](docs/PROJECT_SUMMARY.md)
- [MVP completion notes](docs/MVP_COMPLETE.md)
- [Contributing](docs/CONTRIBUTING.md)

---

*Sweeping the cloud, one bucket at a time.* 🧹👻
