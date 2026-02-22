package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/baseline"
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
	timeout             time.Duration
	baselinePath        string
	updateBaseline      bool
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
	scanCmd.Flags().StringVarP(&scanFlags.outputFormat, "format", "f", "text", "Output format: text, json, or sarif")
	scanCmd.Flags().StringVarP(&scanFlags.outputFile, "output", "o", "", "Output file (default: stdout)")
	scanCmd.Flags().BoolVar(&scanFlags.failOnMissing, "fail-on-missing", false, "Exit with error if missing buckets found")
	scanCmd.Flags().BoolVar(&scanFlags.failOnStale, "fail-on-stale", false, "Exit with error if stale prefixes found")
	scanCmd.Flags().BoolVar(&scanFlags.failOnVersionSprawl, "fail-on-version-sprawl", false, "Exit with error if version sprawl detected")
	scanCmd.Flags().BoolVar(&scanFlags.failOnUnused, "fail-on-unused", false, "Exit with error if unused buckets found")
	scanCmd.Flags().BoolVar(&scanFlags.includeReferences, "include-references", false, "Include detailed reference list in output")
	scanCmd.Flags().BoolVar(&scanFlags.noProgress, "no-progress", false, "Disable progress indicators")
	scanCmd.Flags().DurationVar(&scanFlags.timeout, "timeout", 0, "Total operation timeout (e.g. 5m, 30s). 0 means no timeout")
	scanCmd.Flags().StringVar(&scanFlags.baselinePath, "baseline", "", "Path to previous JSON report for diff comparison")
	scanCmd.Flags().BoolVar(&scanFlags.updateBaseline, "update-baseline", false, "Write current results as the new baseline")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Apply config file defaults for flags not explicitly set
	applyConfigToScanFlags(cmd)

	ctx := context.Background()
	if scanFlags.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, scanFlags.timeout)
		defer cancel()
	}
	start := time.Now()

	// Check if we're running in a terminal (for progress indicators)
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	showProgress := isTTY && !scanFlags.noProgress

	// 1. Scan repository for S3 references
	printStatus("Scanning repository: %s", scanFlags.repoPath)
	repoScanner := scanner.NewRepoScanner(scanFlags.repoPath)
	references, err := repoScanner.Scan(ctx)
	if err != nil {
		return enhanceError("repository scan", err, scanFlags.maxConcurrency)
	}
	printStatus("Found %d S3 references in code", len(references))

	// 2. Initialize S3 client
	printStatus("Initializing AWS S3 client...")
	s3Client, err := s3.NewClient(ctx, scanFlags.awsProfile, scanFlags.awsRegion)
	if err != nil {
		return enhanceError("S3 client initialization", err, scanFlags.maxConcurrency)
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
				slog.Debug("Scan progress", slog.Int("current", current), slog.Int("total", total), slog.String("message", message))
			} else {
				slog.Debug("Scan progress", slog.String("message", message))
			}
		})
	}

	// 4. Inspect AWS S3
	printStatus("Inspecting AWS S3 buckets...")
	bucketInfo, err := inspector.InspectBuckets(ctx, references)
	if err != nil {
		return enhanceError("S3 inspection", err, scanFlags.maxConcurrency)
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
			return enhanceError("output file creation", err, scanFlags.maxConcurrency)
		}
		defer func() { _ = f.Close() }()
		writer = f
	}

	// Generate report
	reporter, err := selectReporter(scanFlags.outputFormat, writer)
	if err != nil {
		return err
	}

	if err := reporter.Generate(reportData); err != nil {
		return enhanceError("report generation", err, scanFlags.maxConcurrency)
	}

	// Baseline comparison
	if scanFlags.baselinePath != "" {
		currentFindings := baseline.FlattenScanFindings(reportData)
		baselineFindings, err := baseline.LoadScanBaseline(scanFlags.baselinePath)
		if err != nil {
			return enhanceError("baseline load", err, scanFlags.maxConcurrency)
		}
		diff := baseline.Diff(currentFindings, baselineFindings)
		slog.Info("Baseline comparison",
			slog.Int("new", len(diff.New)),
			slog.Int("resolved", len(diff.Resolved)),
			slog.Int("unchanged", len(diff.Unchanged)),
		)
	}

	// Write updated baseline if requested
	if scanFlags.updateBaseline && scanFlags.outputFile != "" {
		baselineData, err := json.MarshalIndent(reportData, "", "  ")
		if err != nil {
			return enhanceError("baseline write", err, scanFlags.maxConcurrency)
		}
		if err := os.WriteFile(scanFlags.outputFile, baselineData, 0644); err != nil {
			return enhanceError("baseline write", err, scanFlags.maxConcurrency)
		}
		slog.Info("Updated baseline", slog.String("path", scanFlags.outputFile))
	}

	prefixCount := 0
	prefixes := make(map[string]struct{})
	for _, ref := range references {
		if ref.Prefix == "" {
			continue
		}
		key := ref.Bucket + "/" + ref.Prefix
		if _, exists := prefixes[key]; !exists {
			prefixes[key] = struct{}{}
			prefixCount++
		}
	}
	findingCount := len(analysis.Summary.MissingBuckets) +
		len(analysis.Summary.UnusedBuckets) +
		len(analysis.Summary.MissingPrefixes) +
		len(analysis.Summary.StalePrefixes) +
		len(analysis.Summary.VersionSprawl) +
		len(analysis.Summary.LifecycleMisconfig)
	slog.Info("Scan complete",
		slog.Int("bucket_count", analysis.Summary.TotalBuckets),
		slog.Int("prefix_count", prefixCount),
		slog.Int("finding_count", findingCount),
		slog.Duration("duration", time.Since(start)),
	)

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

func applyConfigToScanFlags(cmd *cobra.Command) {
	if !cmd.Flags().Lookup("aws-region").Changed && cfg.Region != "" {
		scanFlags.awsRegion = cfg.Region
	}
	if !cmd.Flags().Lookup("stale-days").Changed && cfg.StaleDays > 0 {
		scanFlags.staleThresholdDays = cfg.StaleDays
	}
	if !cmd.Flags().Lookup("format").Changed && cfg.Format != "" {
		scanFlags.outputFormat = cfg.Format
	}
	if !cmd.Flags().Lookup("timeout").Changed {
		if d := cfg.TimeoutDuration(); d > 0 {
			scanFlags.timeout = d
		}
	}
}
