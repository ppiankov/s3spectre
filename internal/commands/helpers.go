package commands

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/ppiankov/s3spectre/internal/report"
	"github.com/ppiankov/s3spectre/internal/s3"
	"github.com/ppiankov/s3spectre/internal/scanner"
	"github.com/spf13/cobra"
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

// isExcludedBucket reports whether name matches an exact entry in excludeBuckets
// or is prefixed by any entry in excludePrefixes.
func isExcludedBucket(name string, excludeBuckets, excludePrefixes []string) bool {
	for _, b := range excludeBuckets {
		if name == b {
			return true
		}
	}
	for _, p := range excludePrefixes {
		if p != "" && strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// filterExcludedReferences drops scan references whose bucket matches the config
// exclude lists.
func filterExcludedReferences(refs []scanner.Reference, excludeBuckets, excludePrefixes []string) []scanner.Reference {
	if len(excludeBuckets) == 0 && len(excludePrefixes) == 0 {
		return refs
	}
	filtered := make([]scanner.Reference, 0, len(refs))
	for _, ref := range refs {
		if isExcludedBucket(ref.Bucket, excludeBuckets, excludePrefixes) {
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
}

// filterExcludedBuckets drops discovered buckets matching the config exclude lists.
func filterExcludedBuckets(buckets map[string]*s3.BucketInfo, excludeBuckets, excludePrefixes []string) map[string]*s3.BucketInfo {
	if len(excludeBuckets) == 0 && len(excludePrefixes) == 0 {
		return buckets
	}
	filtered := make(map[string]*s3.BucketInfo, len(buckets))
	for name, b := range buckets {
		if isExcludedBucket(name, excludeBuckets, excludePrefixes) {
			continue
		}
		filtered[name] = b
	}
	return filtered
}

// applyCommonConfigDefaults applies config file defaults for the aws-region,
// format, and timeout flags when the user did not explicitly set them on the
// command line. Shared by scan and discover, which otherwise duplicate this
// logic verbatim.
func applyCommonConfigDefaults(cmd *cobra.Command, region, format *string, timeout *time.Duration) {
	if !cmd.Flags().Lookup("aws-region").Changed && cfg.Region != "" {
		*region = cfg.Region
	}
	if !cmd.Flags().Lookup("format").Changed && cfg.Format != "" {
		*format = cfg.Format
	}
	if !cmd.Flags().Lookup("timeout").Changed {
		if d := cfg.TimeoutDuration(); d > 0 {
			*timeout = d
		}
	}
}

// configureInspectorRegions sets the inspector's region scope from flags and
// prints a status message, using the given message templates so scan and
// discover can each keep their own wording. Shared to avoid the two commands
// drifting on the underlying region-selection logic.
func configureInspectorRegions(inspector *s3.Inspector, s3Client *s3.Client, regions []string, allRegions bool, awsRegion string, multiMsg, allMsg, singleMsg string) {
	if len(regions) > 0 {
		inspector.SetRegions(regions)
		printStatus(multiMsg, strings.Join(regions, ", "))
	} else if allRegions {
		inspector.SetAllRegions(true)
		printStatus(allMsg)
	} else {
		region := awsRegion
		if region == "" {
			region = s3Client.GetRegion()
		}
		printStatus(singleMsg, region)
	}
}

func selectReporter(format string, writer io.Writer) (report.Reporter, error) {
	switch format {
	case "json":
		return report.NewJSONReporter(writer), nil
	case "sarif":
		return report.NewSARIFReporter(writer), nil
	case "spectrehub":
		return report.NewSpectreHubReporter(writer), nil
	case "text":
		return report.NewTextReporter(writer), nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s (supported: text, json, sarif, spectrehub)", format)
	}
}
