package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/ppiankov/s3spectre/internal/analyzer"
)

// TextReporter generates human-readable text reports
type TextReporter struct {
	writer io.Writer
}

// NewTextReporter creates a new text reporter
func NewTextReporter(w io.Writer) *TextReporter {
	return &TextReporter{writer: w}
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	sizes := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), sizes[exp])
}

// Generate generates a text report
func (r *TextReporter) Generate(data Data) error {
	// Header
	if data.Version != "" {
		_, _ = fmt.Fprintf(r.writer, "S3Spectre %s\n", data.Version)
	} else {
		_, _ = fmt.Fprintf(r.writer, "S3Spectre Report\n")
	}
	_, _ = fmt.Fprintf(r.writer, "================\n\n")
	_, _ = fmt.Fprintf(r.writer, "Scan Time: %s\n", data.Timestamp.Format("2006-01-02 15:04:05"))
	_, _ = fmt.Fprintf(r.writer, "Repository: %s\n", data.Config.RepoPath)
	if data.Config.AWSProfile != "" {
		_, _ = fmt.Fprintf(r.writer, "AWS Profile: %s\n", data.Config.AWSProfile)
	}
	if data.Config.AWSRegion != "" {
		_, _ = fmt.Fprintf(r.writer, "AWS Region: %s\n", data.Config.AWSRegion)
	}
	_, _ = fmt.Fprintf(r.writer, "\n")

	// Summary
	r.printSummary(data.Summary)

	// Detailed findings
	r.printFindings(data.Buckets, data.Summary)

	return nil
}

func (r *TextReporter) printSummary(summary analyzer.Summary) {
	_, _ = fmt.Fprintf(r.writer, "Summary\n")
	_, _ = fmt.Fprintf(r.writer, "-------\n")
	_, _ = fmt.Fprintf(r.writer, "Total Buckets Scanned: %d\n", summary.TotalBuckets)
	_, _ = fmt.Fprintf(r.writer, "OK: %d\n", summary.OKBuckets)

	if len(summary.MissingBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.RedString("Missing Buckets"),
			len(summary.MissingBuckets))
	}

	if len(summary.UnusedBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.YellowString("Unused Buckets"),
			len(summary.UnusedBuckets))
	}

	if len(summary.MissingPrefixes) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.YellowString("Missing Prefixes"),
			len(summary.MissingPrefixes))
	}

	if len(summary.StalePrefixes) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.YellowString("Stale Prefixes"),
			len(summary.StalePrefixes))
	}

	if len(summary.VersionSprawl) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.MagentaString("Version Sprawl"),
			len(summary.VersionSprawl))
	}

	if len(summary.LifecycleMisconfig) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.CyanString("Lifecycle Misconfig"),
			len(summary.LifecycleMisconfig))
	}

	_, _ = fmt.Fprintf(r.writer, "\n")
}

func (r *TextReporter) printFindings(buckets map[string]*analyzer.BucketAnalysis, summary analyzer.Summary) {
	// Print missing buckets
	if len(summary.MissingBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.RedString("Missing Buckets"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))
		sort.Strings(summary.MissingBuckets)
		for _, bucket := range summary.MissingBuckets {
			analysis := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.RedString("[MISSING_BUCKET]"),
				bucket)
			if analysis.Message != "" {
				_, _ = fmt.Fprintf(r.writer, "    %s\n", analysis.Message)
			}
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	// Print unused buckets
	if len(summary.UnusedBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.YellowString("Unused Buckets"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))
		sort.Strings(summary.UnusedBuckets)
		for _, bucket := range summary.UnusedBuckets {
			analysis := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.YellowString("[UNUSED_BUCKET]"),
				bucket)
			if analysis.Message != "" {
				_, _ = fmt.Fprintf(r.writer, "    %s\n", analysis.Message)
			}
			if analysis.UnusedScore != nil {
				_, _ = fmt.Fprintf(r.writer, "    Reasons:\n")
				for _, reason := range analysis.UnusedScore.Reasons {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", reason)
				}
			}
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	// Print stale prefixes
	if len(summary.StalePrefixes) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.YellowString("Stale Prefixes"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))
		sort.Strings(summary.StalePrefixes)
		for _, prefixPath := range summary.StalePrefixes {
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.YellowString("[STALE_PREFIX]"),
				prefixPath)
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	// Print missing prefixes
	if len(summary.MissingPrefixes) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.YellowString("Missing Prefixes"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))
		sort.Strings(summary.MissingPrefixes)
		for _, prefixPath := range summary.MissingPrefixes {
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.YellowString("[MISSING_PREFIX]"),
				prefixPath)
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	// Print version sprawl
	if len(summary.VersionSprawl) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.MagentaString("Version Sprawl"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))
		sort.Strings(summary.VersionSprawl)
		for _, bucket := range summary.VersionSprawl {
			analysis := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.MagentaString("[VERSION_SPRAWL]"),
				bucket)
			if analysis.Message != "" {
				_, _ = fmt.Fprintf(r.writer, "    %s\n", analysis.Message)
			}
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	// Print lifecycle misconfigs
	if len(summary.LifecycleMisconfig) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.CyanString("Lifecycle Misconfigurations"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))
		sort.Strings(summary.LifecycleMisconfig)
		for _, bucket := range summary.LifecycleMisconfig {
			analysis := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.CyanString("[LIFECYCLE_MISCONFIG]"),
				bucket)
			if analysis.Message != "" {
				_, _ = fmt.Fprintf(r.writer, "    %s\n", analysis.Message)
			}
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	// Print OK buckets summary
	if summary.OKBuckets > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.GreenString("OK Buckets: %d", summary.OKBuckets))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 50))

		var okBuckets []string
		for name, analysis := range buckets {
			if analysis.Status == analyzer.StatusOK {
				okBuckets = append(okBuckets, name)
			}
		}
		sort.Strings(okBuckets)

		for _, bucket := range okBuckets {
			_, _ = fmt.Fprintf(r.writer, "  %s: %s\n",
				color.GreenString("[OK]"),
				bucket)
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}
}

// GenerateDiscovery generates a text discovery report
func (r *TextReporter) GenerateDiscovery(data DiscoveryData) error {
	// Header
	if data.Version != "" {
		_, _ = fmt.Fprintf(r.writer, "S3Spectre %s — Discovery\n", data.Version)
	} else {
		_, _ = fmt.Fprintf(r.writer, "S3Spectre Discovery Report\n")
	}
	_, _ = fmt.Fprintf(r.writer, "===========================\n\n")
	_, _ = fmt.Fprintf(r.writer, "Scan Time: %s\n", data.Timestamp.Format("2006-01-02 15:04:05"))
	if data.Config.AWSProfile != "" {
		_, _ = fmt.Fprintf(r.writer, "AWS Profile: %s\n", data.Config.AWSProfile)
	}
	if data.Config.AllRegions {
		_, _ = fmt.Fprintf(r.writer, "Scanning: All enabled AWS regions\n")
	} else if len(data.Config.Regions) > 0 {
		_, _ = fmt.Fprintf(r.writer, "Regions: %s\n", strings.Join(data.Config.Regions, ", "))
	}
	_, _ = fmt.Fprintf(r.writer, "Total Regions Scanned: %d\n", data.Summary.TotalRegions)
	_, _ = fmt.Fprintf(r.writer, "\n")

	// Summary
	r.printDiscoverySummary(data.Summary)

	// Detailed findings
	r.printDiscoveryFindings(data.Buckets, data.Summary)

	return nil
}

func (r *TextReporter) printDiscoverySummary(summary analyzer.DiscoverySummary) {
	_, _ = fmt.Fprintf(r.writer, "Summary\n")
	_, _ = fmt.Fprintf(r.writer, "-------\n")
	_, _ = fmt.Fprintf(r.writer, "Total Buckets: %d\n", summary.TotalBuckets)
	_, _ = fmt.Fprintf(r.writer, "Healthy: %d\n", summary.HealthyBuckets)

	if len(summary.UnusedBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.YellowString("Unused"),
			len(summary.UnusedBuckets))
	}

	if len(summary.RiskyBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.RedString("Risky"),
			len(summary.RiskyBuckets))
	}

	if len(summary.InactiveBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.YellowString("Inactive"),
			len(summary.InactiveBuckets))
	}

	if len(summary.VersionSprawl) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s: %d\n",
			color.MagentaString("Version Sprawl"),
			len(summary.VersionSprawl))
	}

	_, _ = fmt.Fprintf(r.writer, "\n")
}

func (r *TextReporter) printDiscoveryFindings(buckets map[string]*analyzer.BucketDiscovery, summary analyzer.DiscoverySummary) {
	// Print unused buckets
	if len(summary.UnusedBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.YellowString("Unused Buckets"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 70))
		sort.Strings(summary.UnusedBuckets)
		for _, bucket := range summary.UnusedBuckets {
			discovery := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s (%s)\n",
				color.YellowString("[UNUSED]"),
				bucket,
				discovery.Region)
			_, _ = fmt.Fprintf(r.writer, "    Risk Score: %d/100\n", discovery.RiskScore)
			if len(discovery.RiskFactors) > 0 {
				_, _ = fmt.Fprintf(r.writer, "    Factors:\n")
				for _, factor := range discovery.RiskFactors {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", factor)
				}
			}
			if len(discovery.Recommendations) > 0 {
				_, _ = fmt.Fprintf(r.writer, "    Recommendations:\n")
				for _, rec := range discovery.Recommendations {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", rec)
				}
			}
			_, _ = fmt.Fprintf(r.writer, "\n")
		}
	}

	// Print risky buckets
	if len(summary.RiskyBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.RedString("Risky Buckets"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 70))
		sort.Strings(summary.RiskyBuckets)
		for _, bucket := range summary.RiskyBuckets {
			discovery := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s (%s)\n",
				color.RedString("[RISKY]"),
				bucket,
				discovery.Region)
			_, _ = fmt.Fprintf(r.writer, "    Risk Score: %d/100\n", discovery.RiskScore)
			if len(discovery.RiskFactors) > 0 {
				_, _ = fmt.Fprintf(r.writer, "    Factors:\n")
				for _, factor := range discovery.RiskFactors {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", factor)
				}
			}
			if len(discovery.Recommendations) > 0 {
				_, _ = fmt.Fprintf(r.writer, "    Recommendations:\n")
				for _, rec := range discovery.Recommendations {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", rec)
				}
			}
			_, _ = fmt.Fprintf(r.writer, "\n")
		}
	}

	// Print inactive buckets
	if len(summary.InactiveBuckets) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.YellowString("Inactive Buckets"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 70))
		sort.Strings(summary.InactiveBuckets)
		for _, bucket := range summary.InactiveBuckets {
			discovery := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s (%s)\n",
				color.YellowString("[INACTIVE]"),
				bucket,
				discovery.Region)
			_, _ = fmt.Fprintf(r.writer, "    Risk Score: %d/100\n", discovery.RiskScore)
			if len(discovery.RiskFactors) > 0 {
				_, _ = fmt.Fprintf(r.writer, "    Factors:\n")
				for _, factor := range discovery.RiskFactors {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", factor)
				}
			}
			_, _ = fmt.Fprintf(r.writer, "\n")
		}
	}

	// Print version sprawl
	if len(summary.VersionSprawl) > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.MagentaString("Version Sprawl"))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 70))
		sort.Strings(summary.VersionSprawl)
		for _, bucket := range summary.VersionSprawl {
			discovery := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s (%s)\n",
				color.MagentaString("[VERSION_SPRAWL]"),
				bucket,
				discovery.Region)

			// Show size information
			if discovery.BucketInfo != nil {
				if discovery.BucketInfo.TotalVersionSize > 0 {
					_, _ = fmt.Fprintf(r.writer, "    Total Size (all versions): %s (%d versions)\n",
						formatBytes(discovery.BucketInfo.TotalVersionSize),
						discovery.BucketInfo.VersionCount)
				}
				if discovery.BucketInfo.TotalSize > 0 && discovery.BucketInfo.TotalVersionSize > discovery.BucketInfo.TotalSize {
					overhead := discovery.BucketInfo.TotalVersionSize - discovery.BucketInfo.TotalSize
					_, _ = fmt.Fprintf(r.writer, "    Version Overhead: %s (%.1f%% of total)\n",
						formatBytes(overhead),
						float64(overhead)/float64(discovery.BucketInfo.TotalVersionSize)*100)
				}
			}

			if len(discovery.RiskFactors) > 0 {
				_, _ = fmt.Fprintf(r.writer, "    Factors:\n")
				for _, factor := range discovery.RiskFactors {
					_, _ = fmt.Fprintf(r.writer, "      - %s\n", factor)
				}
			}
			_, _ = fmt.Fprintf(r.writer, "\n")
		}
	}

	// Print healthy buckets summary
	if summary.HealthyBuckets > 0 {
		_, _ = fmt.Fprintf(r.writer, "%s\n", color.GreenString("Healthy Buckets: %d", summary.HealthyBuckets))
		_, _ = fmt.Fprintf(r.writer, "%s\n", strings.Repeat("-", 70))

		var healthyBuckets []string
		for name, discovery := range buckets {
			if discovery.Status == analyzer.StatusOK {
				healthyBuckets = append(healthyBuckets, name)
			}
		}
		sort.Strings(healthyBuckets)

		// Show first 10 healthy buckets
		displayCount := len(healthyBuckets)
		if displayCount > 10 {
			displayCount = 10
		}

		for i := 0; i < displayCount; i++ {
			bucket := healthyBuckets[i]
			discovery := buckets[bucket]
			_, _ = fmt.Fprintf(r.writer, "  %s: %s (%s)\n",
				color.GreenString("[OK]"),
				bucket,
				discovery.Region)
		}

		if len(healthyBuckets) > 10 {
			_, _ = fmt.Fprintf(r.writer, "  ... and %d more\n", len(healthyBuckets)-10)
		}

		_, _ = fmt.Fprintf(r.writer, "\n")
	}
}
