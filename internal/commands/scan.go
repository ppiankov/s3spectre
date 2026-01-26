package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/report"
	"github.com/ppiankov/s3spectre/internal/s3"
	"github.com/ppiankov/s3spectre/internal/scanner"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var scanFlags struct {
	repoPath            string
	awsProfile          string
	awsRegion           string
	allRegions          bool
	regions             []string
	staleThresholdDays  int
	unusedThresholdDays int
	checkUnused         bool
	maxConcurrency      int
	outputFormat        string
	outputFile          string
	failOnMissing       bool
	failOnStale         bool
	failOnVersionSprawl bool
	failOnUnused        bool
	includeReferences   bool
	noProgress          bool
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan repository and AWS S3 for bucket drift",
	Long: `Scans your codebase for S3 bucket references, queries AWS S3 for actual
bucket state, and detects missing buckets, unused buckets, stale prefixes,
version sprawl, and lifecycle misconfigurations.`,
	RunE: runScan,
}

func init() {
	scanCmd.Flags().StringVarP(&scanFlags.repoPath, "repo", "r", ".", "Path to repository to scan")
	scanCmd.Flags().StringVar(&scanFlags.awsProfile, "aws-profile", "", "AWS profile to use")
	scanCmd.Flags().StringVar(&scanFlags.awsRegion, "aws-region", "", "AWS region (defaults to profile default)")
	scanCmd.Flags().BoolVar(&scanFlags.allRegions, "all-regions", true, "Scan all enabled AWS regions")
	scanCmd.Flags().StringSliceVar(&scanFlags.regions, "regions", nil, "Specific regions to scan (comma-separated)")
	scanCmd.Flags().IntVar(&scanFlags.staleThresholdDays, "stale-days", 90, "Days threshold for stale prefix detection")
	scanCmd.Flags().IntVar(&scanFlags.unusedThresholdDays, "unused-threshold-days", 180, "Days threshold for unused bucket detection")
	scanCmd.Flags().BoolVar(&scanFlags.checkUnused, "check-unused", false, "Enable unused bucket detection")
	scanCmd.Flags().IntVar(&scanFlags.maxConcurrency, "concurrency", 10, "Max concurrent S3 API calls")
	scanCmd.Flags().StringVarP(&scanFlags.outputFormat, "format", "f", "text", "Output format: text or json")
	scanCmd.Flags().StringVarP(&scanFlags.outputFile, "output", "o", "", "Output file (default: stdout)")
	scanCmd.Flags().BoolVar(&scanFlags.failOnMissing, "fail-on-missing", false, "Exit with error if missing buckets found")
	scanCmd.Flags().BoolVar(&scanFlags.failOnStale, "fail-on-stale", false, "Exit with error if stale prefixes found")
	scanCmd.Flags().BoolVar(&scanFlags.failOnVersionSprawl, "fail-on-version-sprawl", false, "Exit with error if version sprawl detected")
	scanCmd.Flags().BoolVar(&scanFlags.failOnUnused, "fail-on-unused", false, "Exit with error if unused buckets found")
	scanCmd.Flags().BoolVar(&scanFlags.includeReferences, "include-references", false, "Include detailed reference list in output")
	scanCmd.Flags().BoolVar(&scanFlags.noProgress, "no-progress", false, "Disable progress indicators")
}

func runScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if we're running in a terminal (for progress indicators)
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	showProgress := isTTY && !scanFlags.noProgress

	// 1. Scan repository for S3 references
	printStatus("Scanning repository: %s", scanFlags.repoPath)
	repoScanner := scanner.NewRepoScanner(scanFlags.repoPath)
	references, err := repoScanner.Scan(ctx)
	if err != nil {
		return enhanceError("repository scan", err)
	}
	printStatus("Found %d S3 references in code", len(references))

	// 2. Initialize S3 client
	printStatus("Initializing AWS S3 client...")
	s3Client, err := s3.NewClient(ctx, scanFlags.awsProfile, scanFlags.awsRegion)
	if err != nil {
		return enhanceError("S3 client initialization", err)
	}

	// 3. Configure inspector
	inspector := s3.NewInspector(s3Client, scanFlags.maxConcurrency)

	// Set up regions
	if len(scanFlags.regions) > 0 {
		// Specific regions provided
		inspector.SetRegions(scanFlags.regions)
		printStatus("Scanning regions: %s", strings.Join(scanFlags.regions, ", "))
	} else if scanFlags.allRegions {
		// Scan all regions
		inspector.SetAllRegions(true)
		printStatus("Scanning all enabled AWS regions")
	} else {
		// Single region (default)
		region := scanFlags.awsRegion
		if region == "" {
			region = s3Client.GetRegion()
		}
		printStatus("Scanning region: %s", region)
	}

	// Set up progress callback
	if showProgress {
		inspector.SetProgressCallback(func(current, total int, message string) {
			if total > 0 {
				fmt.Fprintf(os.Stderr, "\r[%d/%d] %s", current, total, message)
			} else {
				fmt.Fprintf(os.Stderr, "\r%s", message)
			}
		})
	}

	// 4. Inspect AWS S3
	printStatus("Inspecting AWS S3 buckets...")
	bucketInfo, err := inspector.InspectBuckets(ctx, references)
	if err != nil {
		return enhanceError("S3 inspection", err)
	}
	if showProgress {
		fmt.Fprintf(os.Stderr, "\n") // Clear progress line
	}
	printStatus("Inspected %d buckets", len(bucketInfo))

	// 5. Analyze drift
	printStatus("Analyzing drift...")
	config := analyzer.Config{
		StaleThresholdDays:   scanFlags.staleThresholdDays,
		UnusedThresholdDays:  scanFlags.unusedThresholdDays,
		CheckUnused:          scanFlags.checkUnused,
		UnusedScoreThreshold: 150, // Default threshold
	}
	analysis := analyzer.Analyze(references, bucketInfo, config)

	// 6. Generate report
	reportData := report.Data{
		Tool:      "s3spectre",
		Version:   GetVersion(),
		Timestamp: time.Now(),
		Config: report.Config{
			RepoPath:           scanFlags.repoPath,
			AWSProfile:         scanFlags.awsProfile,
			AWSRegion:          s3Client.GetRegion(),
			StaleThresholdDays: scanFlags.staleThresholdDays,
		},
		Summary: analysis.Summary,
		Buckets: analysis.Buckets,
	}

	if scanFlags.includeReferences {
		reportData.References = references
	}

	// Determine output writer
	writer := os.Stdout
	if scanFlags.outputFile != "" {
		f, err := os.Create(scanFlags.outputFile)
		if err != nil {
			return enhanceError("output file creation", err)
		}
		defer f.Close()
		writer = f
	}

	// Generate report
	var reporter report.Reporter
	switch scanFlags.outputFormat {
	case "json":
		reporter = report.NewJSONReporter(writer)
	case "text":
		reporter = report.NewTextReporter(writer)
	default:
		return fmt.Errorf("unsupported output format: %s (supported: text, json)", scanFlags.outputFormat)
	}

	if err := reporter.Generate(reportData); err != nil {
		return enhanceError("report generation", err)
	}

	// Check exit conditions
	if scanFlags.failOnMissing && len(analysis.Summary.MissingBuckets) > 0 {
		return fmt.Errorf("found %d missing buckets", len(analysis.Summary.MissingBuckets))
	}
	if scanFlags.failOnStale && len(analysis.Summary.StalePrefixes) > 0 {
		return fmt.Errorf("found %d stale prefixes", len(analysis.Summary.StalePrefixes))
	}
	if scanFlags.failOnVersionSprawl && len(analysis.Summary.VersionSprawl) > 0 {
		return fmt.Errorf("found %d buckets with version sprawl", len(analysis.Summary.VersionSprawl))
	}
	if scanFlags.failOnUnused && len(analysis.Summary.UnusedBuckets) > 0 {
		return fmt.Errorf("found %d unused buckets", len(analysis.Summary.UnusedBuckets))
	}

	return nil
}

// printStatus prints a status message to stderr
func printStatus(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// enhanceError enhances an error with additional context and helpful suggestions
func enhanceError(operation string, err error) error {
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
			"Original error: %w", operation, scanFlags.maxConcurrency, err)
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
