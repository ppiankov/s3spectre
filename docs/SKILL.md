# s3spectre

S3 bucket exposure and misconfiguration scanner.

## Install

```
brew install ppiankov/tap/s3spectre
```

Or via Go:

```
go install github.com/ppiankov/s3spectre/cmd/s3spectre@latest
```

## Commands

### s3spectre scan

Scans S3 buckets for security findings.

**Flags:**
- `--output json` — output as JSON (spectre/v1 envelope)
- `--output sarif` — SARIF format for CI integration
- `--output spectrehub` — SpectreHub aggregator format
- `--baseline path` — suppress known findings

**JSON output:**
```json
{
  "version": "spectre/v1",
  "scanner": "s3spectre",
  "target": "S3 buckets",
  "findings": [
    {
      "id": "FIND-001",
      "severity": "high",
      "title": "finding description",
      "resource": "resource identifier",
      "detail": "detailed explanation"
    }
  ],
  "summary": {
    "total": 1,
    "critical": 0,
    "high": 1,
    "medium": 0,
    "low": 0
  }
}
```

**Exit codes:**
- 0: scan complete, no findings
- 1: scan complete, findings detected
- 2: scan failed (connectivity, auth, config error)

### s3spectre init

Initialize configuration with sensible defaults.

**Exit codes:**
- 0: config created
- 1: config already exists or error

## Handoffs

- Output: spectre/v1 JSON envelope. Next: spectrehub for aggregation across scanners.
- Output: SARIF. Next: CI security gates.
- Refused questions: how to fix findings, whether to remediate, risk acceptance decisions.

## What this does NOT do

- Does not remediate or modify S3 buckets — scan is read-only
- Does not store findings or manage a findings database
- Does not replace dedicated S3 buckets monitoring — point-in-time security audit only

## Failure Modes

- Authentication failure: returns exit code 2. Distrust: all findings fields. Safe fallback: report scan failure, do not cache.
- Network timeout: returns exit code 2. Distrust: completeness of findings. Safe fallback: partial results with warning.
- Rate limiting: returns partial findings with truncation warning. Distrust: summary counts.

## Parsing examples

```bash
s3spectre scan --output json | jq '.summary'
s3spectre scan --output json | jq '.findings[] | select(.severity == "critical")'
```

---

This tool follows the [Agent-Native CLI Convention](https://ancc.dev). Validate with: `ancc validate .`
