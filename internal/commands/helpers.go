package commands

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/ppiankov/s3spectre/internal/report"
)

func printStatus(format string, args ...interface{}) {
	slog.Info(fmt.Sprintf(format, args...))
}

// enhanceError enhances an error with additional context and helpful suggestions
func enhanceError(operation string, err error, concurrency int) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Provide helpful suggestions for common errors
	if strings.Contains(errMsg, "NoCredentialProviders") || strings.Contains(errMsg, "no valid credentials") {
		return fmt.Errorf("%s failed: No AWS credentials found.\n"+
			"Solutions:\n"+
			"  - Set AWS_PROFILE environment variable\n"+
			"  - Use --aws-profile flag\n"+
			"  - Configure AWS credentials with 'aws configure'\n"+
			"Original error: %w", operation, err)
	}

	if strings.Contains(errMsg, "AccessDenied") || strings.Contains(errMsg, "Access Denied") {
		return fmt.Errorf("%s failed: Access Denied.\n"+
			"Solutions:\n"+
			"  - Check IAM permissions for S3 operations\n"+
			"  - Ensure you have s3:ListBucket, s3:GetBucketLocation, s3:GetBucketVersioning permissions\n"+
			"  - Verify the correct AWS profile is being used\n"+
			"Original error: %w", operation, err)
	}

	if strings.Contains(errMsg, "RequestLimitExceeded") || strings.Contains(errMsg, "SlowDown") {
		return fmt.Errorf("%s failed: AWS rate limit exceeded.\n"+
			"Solutions:\n"+
			"  - Reduce concurrency with --concurrency flag (current: %d)\n"+
			"  - Wait a few seconds and try again\n"+
			"Original error: %w", operation, concurrency, err)
	}

	if strings.Contains(errMsg, "no such file or directory") {
		return fmt.Errorf("%s failed: Repository path not found.\n"+
			"Solutions:\n"+
			"  - Check the --repo path is correct\n"+
			"  - Ensure the directory exists and is readable\n"+
			"Original error: %w", operation, err)
	}

	// Default error with context
	return fmt.Errorf("%s failed: %w", operation, err)
}

func selectReporter(format string, writer io.Writer) (report.Reporter, error) {
	switch format {
	case "json":
		return report.NewJSONReporter(writer), nil
	case "sarif":
		return report.NewSARIFReporter(writer), nil
	case "text":
		return report.NewTextReporter(writer), nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s (supported: text, json, sarif)", format)
	}
}
