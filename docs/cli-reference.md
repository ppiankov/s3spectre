## Philosophy

*Principiis obsta* -- resist the beginnings.

Infrastructure drift is not a detection problem. It is a structural problem. By the time a missing bucket breaks a deployment, the damage is done. S3Spectre is designed to surface these conditions early -- in CI, in code review, in scheduled audits -- so they can be addressed before they matter.

The tool presents evidence and lets humans decide. It does not auto-remediate, does not guess intent, and does not assign confidence scores where deterministic checks suffice.


## Installation

```bash
# Homebrew
brew install ppiankov/tap/s3spectre

# Docker
docker pull ghcr.io/ppiankov/s3spectre:latest

# From source
git clone https://github.com/ppiankov/s3spectre.git
cd s3spectre && make build

# Windows: download s3spectre_<version>_windows_amd64.zip (or _arm64) from
# https://github.com/ppiankov/s3spectre/releases, extract, add to PATH.
# Alternatively, with Go installed:
go install github.com/ppiankov/s3spectre/cmd/s3spectre@latest
```


## Configuration file

S3Spectre reads `.s3spectre.yaml` (or `.s3spectre.yml`) from the current directory, falling back to the user's home directory. Explicit CLI flags always take precedence over config file values.

```yaml
region: us-east-1
exclude_buckets:
  - legacy-shared-bucket
  - aws-cloudtrail-logs-123456789012-abcd1234
exclude_prefixes:
  - tmp-
  - sandbox-
stale_days: 90
format: json
timeout: 5m
```

| Key | Applies to | Description |
|-----|-----------|--------------|
| `region` | scan, discover | Default AWS region when `--aws-region` is not set |
| `exclude_buckets` | scan, discover | Exact bucket names to omit from all findings |
| `exclude_prefixes` | scan, discover | Bucket-name prefixes to omit from all findings |
| `stale_days` | scan | Default for `--stale-days` |
| `format` | scan, discover | Default for `--format` |
| `timeout` | scan, discover | Default for `--timeout` |
| `risk_threshold` | discover | Default for `--risk-threshold` |
| `public_bucket_allowlist_patterns` | discover | Naming substrings (case-insensitive) for intentionally-public buckets; extends, does not replace, the built-in defaults (`public`, `webview`, `-cdn`, `-landing`) |


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
| `--format, -f` | `text` | Output format: `text`, `json`, `sarif`, `spectrehub`, or `markdown` |
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
| `--risk-threshold` | `100` | Risk score at which a bucket is flagged unused/risky/inactive |
| `--check-encryption` | `false` | Flag missing encryption |
| `--check-public` | `false` | Flag public access |
| `--estimate-cost` | `false` | Approximate monthly USD cost of version-sprawl overhead, and of inactive/unused bucket storage |
| `--suggest-lifecycle-policy` | `false` | Suggest a deterministic lifecycle-rule snippet (JSON + Terraform) for version-sprawl findings; informational only, never applied |
| `--group-by-tag` | | Roll up the discover summary by the given bucket tag key (e.g. `Team`); buckets missing the tag are grouped as `untagged` |
| `--concurrency` | `10` | Max concurrent S3 API calls |
| `--format, -f` | `text` | Output format: `text`, `json`, `sarif`, `spectrehub`, or `markdown` |
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
- **Regex-based code scanning.** The scanner uses pattern matching, not AST parsing. It will miss dynamically constructed bucket names and may produce false positives on commented-out code. In source code and JSON files specifically, bucket-name assignments (`bucket = "..."`, `BUCKET_NAME: "..."`) are only recognized when the value is a quoted string literal -- an unquoted code expression (a variable, an attribute access, a function call) is never mistaken for a literal bucket name -- and a small set of common documentation placeholders (`bucket`, `my-bucket`, `example-bucket`, and similar) are never reported as real references there, since real S3 bucket names are globally unique and no production bucket is actually named one of these bare words. This quote-requirement and placeholder filtering does not yet extend to `.env`, YAML, or Terraform scanning, which use their own separate patterns (`.env`'s `KEY=value` convention is legitimately unquoted, so the same fix doesn't directly apply there).
- **No cost estimation by default.** `discover --estimate-cost` gives an approximate monthly USD figure for version-sprawl storage overhead, and separately for the full storage of buckets flagged inactive/unused, using an embedded on-demand pricing table (S3 Standard only, no request/data-transfer charges). Risky/OK buckets and scan mode remain unpriced. The summary's `total_estimated_cost_usd` is the sum of these two priced scopes only, not a full-account cost estimate.
- **`--group-by-tag` rollup is a summary view, not a filter.** It groups the same findings by a tag value for an ownership-level rollup (bucket count, summed risk score, and average risk score per bucket); it does not change which buckets are scanned or scored. Buckets missing the tag land under `untagged`, never dropped. If a bucket happens to carry the tag value literally set to `untagged`, it merges into the same rollup entry as buckets missing the tag entirely -- an accepted edge case, not disambiguated.
- **`--group-by-tag`'s risk score is reported two ways: summed and averaged.** The summed `risk_score` scales with how many buckets a tag value owns, so it favors large groups even when their buckets are individually low-severity. `average_risk_score` (summed risk score divided by bucket count) surfaces per-bucket severity instead, so a small group of consistently bad buckets isn't lost in the numbers next to a larger group of mostly-fine ones. The rendered list/table order is unchanged -- still sorted by descending summed `risk_score`, not by average -- so read `average_risk_score` explicitly per row rather than relying on position to reveal it.
- **Default-KMS-key detection is a scoring nuance, not a compliance certification.** `--check-encryption` adds a small (15-point) factor when a bucket uses `aws:kms` with the AWS-managed default key (`alias/aws/s3`) rather than a customer-managed key. This does not check any specific compliance framework's actual requirements -- it surfaces the distinction so an operator can judge it against their own framework.
- **Public-access naming allowlist is a severity reduction, not a suppression.** Matching is a case-insensitive substring test, not a glob -- a bucket name containing `public`, `webview`, `-cdn`, or `-landing` (built-in defaults) or any `public_bucket_allowlist_patterns` config entry still gets flagged public at half severity and always appears in the `public_buckets` inventory -- nothing is silently dropped from evidence.
- **Lifecycle-policy suggestions are advisory only.** `discover --suggest-lifecycle-policy` generates a JSON and Terraform snippet from already-collected bucket metadata for version-sprawl findings. s3spectre never calls any AWS write API and never applies the suggestion; it is text for an operator to review and apply through their own deployment path.
- **IAM permissions required.** Needs `s3:ListBucket`, `s3:ListAllMyBuckets`, `s3:GetBucketLocation`, `s3:GetBucketVersioning`, `s3:GetLifecycleConfiguration`, `s3:GetBucketTagging`, `s3:GetEncryptionConfiguration`, and `s3:GetBucketPublicAccessBlock`. `discover` always calls the encryption and public-access-block APIs (not just when `--check-encryption`/`--check-public` are passed -- those flags only gate whether the analyzer scores the result), so missing either permission is not purely cosmetic. Missing permissions for the core listing/inspection calls produce access-denied errors; missing permissions specifically for `GetEncryptionConfiguration`/`GetBucketPublicAccessBlock` instead silently omit those two fields for the affected bucket (`--check-encryption`/`--check-public` findings for it), rather than failing the whole scan.
- **No real-time monitoring.** S3Spectre is a point-in-time scanner, not a daemon. Run it in CI or on a schedule.
- **Single AWS account.** Cross-account scanning is not supported.
- **Progress line artifacts.** The TTY progress indicator uses carriage return without clearing the full line, so shorter bucket names leave trailing characters from the previous name. Cosmetic only.
- **AWS-managed bucket names are exempt from unused/stale advice.** Buckets matching known AWS-managed naming conventions (`aws-cloudtrail-logs-*`, `aws-config-bucket-*`, `elasticloadbalancing-*`, `cf-templates-*`) never get "delete if not needed" recommendations from age, inactivity, or emptiness alone -- deleting them would break the owning AWS service. They are still flagged normally for encryption, public access, and version sprawl.
- **Legacy region-constraint values are mapped, not guessed.** Very old buckets can still return historic `GetBucketLocation` values (e.g. `EU` for `eu-west-1`) instead of the modern region code; s3spectre maps the one AWS-documented case (`EU`) to its modern equivalent. `--regions`-scoped (and default all-regions) `discover` fails open for any other value that doesn't look like a real AWS region name -- the bucket stays visible with a logged warning instead of being silently dropped, since guessing at an unrecognized value would risk misplacing it.


## Roadmap

- Deep prefix scanning with pagination
- Replication rule validation
- IAM access path analysis
- Naming convention enforcement
- Historical trend tracking
- SpectreHub integration for cross-tool correlation

