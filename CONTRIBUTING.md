# Contributing to S3Spectre

Thank you for considering contributing to S3Spectre! This document outlines the process for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/s3spectre`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes
6. Commit and push
7. Create a Pull Request

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make
- AWS CLI (for testing)
- golangci-lint (for linting)

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Code Formatting

```bash
make fmt
```

### Linting

```bash
make lint
```

## Project Structure

```
s3spectre/
├── cmd/s3spectre/          # CLI entry point
├── internal/
│   ├── commands/           # Cobra CLI commands
│   ├── scanner/            # Repository scanning
│   ├── s3/                 # AWS S3 integration
│   ├── analyzer/           # Drift analysis
│   └── report/             # Report generation
├── examples/               # Example repositories
└── docs/                   # Documentation
```

## Contribution Areas

We welcome contributions in these areas:

### 1. Scanner Enhancements

Add support for new file formats or patterns:
- CloudFormation improvements
- Pulumi support
- CDK support
- Language-specific SDK patterns

Example: Add a new scanner in `internal/scanner/`

### 2. Analysis Improvements

Enhance drift detection:
- Better stale prefix heuristics
- Cost estimation
- Security analysis (public buckets, encryption)
- IAM policy analysis

Example: Extend `internal/analyzer/analyzer.go`

### 3. AWS Integration

Improve S3 API integration:
- Support for more S3 features
- Better error handling
- Retry logic
- Rate limiting

Example: Enhance `internal/s3/inspector.go`

### 4. Reporting

Add new report formats or improve existing ones:
- HTML reports
- CSV exports
- Integration with other tools
- Better visualizations

Example: Add new reporter in `internal/report/`

### 5. Documentation

- Improve README
- Add tutorials
- Write blog posts
- Create videos

### 6. Testing

- Add unit tests
- Add integration tests
- Improve test coverage

## Coding Guidelines

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Pass `golangci-lint` checks
- Write clear, concise comments
- Keep functions small and focused

### Error Handling

- Always check errors
- Wrap errors with context using `fmt.Errorf`
- Don't panic in library code

```go
// Good
if err != nil {
    return fmt.Errorf("failed to list buckets: %w", err)
}

// Bad
if err != nil {
    panic(err)
}
```

### Naming Conventions

- Use descriptive names
- Follow Go naming conventions
- Exported names should be clear to external users

### Concurrency

- Use goroutines judiciously
- Always use proper synchronization (mutex, channels)
- Respect the `--concurrency` flag
- Test concurrent code carefully

### Testing

- Write tests for new features
- Maintain or improve test coverage
- Use table-driven tests where appropriate

```go
func TestBucketAnalyzer(t *testing.T) {
    tests := []struct {
        name     string
        bucket   string
        expected Status
    }{
        {"exists", "my-bucket", StatusOK},
        {"missing", "old-bucket", StatusMissingBucket},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Commit Messages

Use clear, descriptive commit messages:

```
Add CloudFormation scanner support

- Implement CloudFormation template parsing
- Detect S3 bucket resources
- Extract bucket properties
- Add tests for CF scanner

Fixes #123
```

Format:
- First line: Brief summary (50 chars or less)
- Blank line
- Detailed description
- Reference issues/PRs

## Pull Request Process

1. **Update documentation**: Update README if needed
2. **Add tests**: Ensure new code is tested
3. **Run checks**: `make test && make lint`
4. **Update CHANGELOG**: Add entry for your change
5. **Create PR**: With clear description
6. **Respond to feedback**: Address review comments

## Code Review

All submissions require review. We'll review for:
- Code quality
- Test coverage
- Documentation
- Performance
- Security

## SpectreHub Compatibility

When making changes, ensure compatibility with SpectreHub:
- JSON output format must match schema
- Include `tool`, `version`, `timestamp` fields
- Follow Spectre family conventions

## Questions?

- Open an issue for discussion
- Join discussions in existing issues
- Reach out to maintainers

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to S3Spectre! Every contribution helps make infrastructure management better for everyone. 🧹👻
