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
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var discoverFlags struct {
	awsProfile       string
	awsRegion        string
	allRegions       bool
	regions          []string
	ageThresholdDays int
	inactiveDays     int
	checkEncryption  bool
	checkPublic      bool
	maxConcurrency   int
	outputFormat     string
	outputFile       string
	failOnUnused     bool
	failOnRisky      bool
	noProgress       bool
}

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover and analyze all S3 buckets in AWS account",
	Long: `Discovers all S3 buckets in your AWS account without requiring code references.
Analyzes buckets for unused resources, security risks, and configuration issues.`,
	RunE: runDiscover,
}

func init() {
	discoverCmd.Flags().StringVar(&discoverFlags.awsProfile, "aws-profile", "", "AWS profile to use")
	discoverCmd.Flags().StringVar(&discoverFlags.awsRegion, "aws-region", "", "AWS region (single region mode)")
	discoverCmd.Flags().BoolVar(&discoverFlags.allRegions, "all-regions", true, "Scan all enabled AWS regions")
	discoverCmd.Flags().StringSliceVar(&discoverFlags.regions, "regions", nil, "Specific regions to scan (comma-separated)")
	discoverCmd.Flags().IntVar(&discoverFlags.ageThresholdDays, "age-threshold-days", 365, "Buckets older than X days are flagged")
	discoverCmd.Flags().IntVar(&discoverFlags.inactiveDays, "inactive-days", 180, "No activity for X days is flagged")
	discoverCmd.Flags().BoolVar(&discoverFlags.checkEncryption, "check-encryption", false, "Check for missing encryption")
	discoverCmd.Flags().BoolVar(&discoverFlags.checkPublic, "check-public", false, "Check for public access")
	discoverCmd.Flags().IntVar(&discoverFlags.maxConcurrency, "concurrency", 10, "Max concurrent S3 API calls")
	discoverCmd.Flags().StringVarP(&discoverFlags.outputFormat, "format", "f", "text", "Output format: text or json")
	discoverCmd.Flags().StringVarP(&discoverFlags.outputFile, "output", "o", "", "Output file (default: stdout)")
	discoverCmd.Flags().BoolVar(&discoverFlags.failOnUnused, "fail-on-unused", false, "Exit with error if unused buckets found")
	discoverCmd.Flags().BoolVar(&discoverFlags.failOnRisky, "fail-on-risky", false, "Exit with error if risky buckets found")
	discoverCmd.Flags().BoolVar(&discoverFlags.noProgress, "no-progress", false, "Disable progress indicators")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Check if we're running in a terminal
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	showProgress := isTTY && !discoverFlags.noProgress

	// Initialize S3 client
	printStatus("Initializing AWS S3 client...")
	s3Client, err := s3.NewClient(ctx, discoverFlags.awsProfile, discoverFlags.awsRegion)
	if err != nil {
		return enhanceError("S3 client initialization", err, discoverFlags.maxConcurrency)
	}

	// Configure inspector
	inspector := s3.NewInspector(s3Client, discoverFlags.maxConcurrency)

	// Set up regions
	if len(discoverFlags.regions) > 0 {
		inspector.SetRegions(discoverFlags.regions)
		printStatus("Discovering buckets in regions: %s", strings.Join(discoverFlags.regions, ", "))
	} else if discoverFlags.allRegions {
		inspector.SetAllRegions(true)
		printStatus("Discovering buckets across all enabled AWS regions")
	} else {
		region := discoverFlags.awsRegion
		if region == "" {
			region = s3Client.GetRegion()
		}
		printStatus("Discovering buckets in region: %s", region)
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

	// Discover all buckets
	printStatus("Discovering S3 buckets...")
	buckets, err := inspector.DiscoverAllBuckets(ctx)
	if err != nil {
		return enhanceError("bucket discovery", err, discoverFlags.maxConcurrency)
	}
	if showProgress {
		fmt.Fprintf(os.Stderr, "\n")
	}
	printStatus("Discovered %d buckets", len(buckets))

	// Analyze with discovery heuristics
	printStatus("Analyzing buckets...")
	config := analyzer.DiscoveryConfig{
		AgeThresholdDays:        discoverFlags.ageThresholdDays,
		InactivityThresholdDays: discoverFlags.inactiveDays,
		CheckEncryption:         discoverFlags.checkEncryption,
		CheckPublicAccess:       discoverFlags.checkPublic,
		RiskScoreThreshold:      100, // Default threshold
	}
	results := analyzer.AnalyzeDiscovery(buckets, config)

	// Generate report
	reportData := report.DiscoveryData{
		Tool:      "s3spectre",
		Version:   GetVersion(),
		Timestamp: time.Now(),
		Config: report.DiscoveryConfig{
			AWSProfile:              discoverFlags.awsProfile,
			AllRegions:              discoverFlags.allRegions,
			Regions:                 discoverFlags.regions,
			AgeThresholdDays:        discoverFlags.ageThresholdDays,
			InactivityThresholdDays: discoverFlags.inactiveDays,
			CheckEncryption:         discoverFlags.checkEncryption,
			CheckPublicAccess:       discoverFlags.checkPublic,
		},
		Summary: results.Summary,
		Buckets: results.Buckets,
	}

	// Determine output writer
	writer := os.Stdout
	if discoverFlags.outputFile != "" {
		f, err := os.Create(discoverFlags.outputFile)
		if err != nil {
			return enhanceError("output file creation", err, discoverFlags.maxConcurrency)
		}
		defer f.Close()
		writer = f
	}

	// Generate report
	var reporter report.Reporter
	switch discoverFlags.outputFormat {
	case "json":
		reporter = report.NewJSONReporter(writer)
	case "text":
		reporter = report.NewTextReporter(writer)
	default:
		return fmt.Errorf("unsupported output format: %s (supported: text, json)", discoverFlags.outputFormat)
	}

	if err := reporter.GenerateDiscovery(reportData); err != nil {
		return enhanceError("report generation", err, discoverFlags.maxConcurrency)
	}

	// Check exit conditions
	if discoverFlags.failOnUnused && len(results.Summary.UnusedBuckets) > 0 {
		return fmt.Errorf("found %d unused buckets", len(results.Summary.UnusedBuckets))
	}
	if discoverFlags.failOnRisky && len(results.Summary.RiskyBuckets) > 0 {
		return fmt.Errorf("found %d risky buckets", len(results.Summary.RiskyBuckets))
	}

	return nil
}
