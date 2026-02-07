# Project: s3spectre

## Commands
- `make build` — Build binary to ./bin/s3spectre
- `make test` — Run tests with -race flag
- `make lint` — Run golangci-lint
- `make fmt` — Format with gofmt/goimports
- `make clean` — Clean build artifacts

## Architecture
- Entry: cmd/s3spectre/main.go — minimal, single Execute() call delegates to internal/commands
- commands — Cobra CLI commands (scan, discover, version) and shared helpers
- scanner — Repository scanning: regex, YAML, Terraform, JSON, .env parsers
- s3 — AWS S3 client wrapper, concurrent bucket/prefix inspector with retry
- analyzer — Drift analysis (scan mode) and risk scoring (discover mode)
- report — Text and JSON output generation

## Conventions
- Minimal main.go — single Execute() call
- Internal packages: short single-word names (scanner, s3, analyzer, report, commands)
- Struct-based domain models with json tags
- Standard Go formatting (gofmt/goimports)
- Package-level compiled regexes via regexp.MustCompile in var blocks
- All S3 API calls go through context-aware methods with retry and exponential backoff

## Anti-Patterns
- NEVER use custom string functions when stdlib exists (strings.Contains, strings.ToLower, etc.)
- NEVER compile regexes inside functions — use package-level var declarations
- NEVER make AWS calls without context and retry logic
- NEVER skip error handling — always check returned errors
- NEVER use init() functions unless absolutely necessary
- NEVER use global mutable state

## Verification
- Run `make test` after code changes (includes -race)
- Run `make lint` before marking complete
- Run `go vet ./...` for suspicious constructs
