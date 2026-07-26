package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ppiankov/s3spectre/internal/analyzer"
)

// MarkdownReporter generates GitHub-flavored Markdown reports -- no ANSI
// color codes -- suitable for pasting into PR comments or chat messages.
type MarkdownReporter struct {
	writer io.Writer
}

// NewMarkdownReporter creates a new Markdown reporter.
func NewMarkdownReporter(w io.Writer) *MarkdownReporter {
	return &MarkdownReporter{writer: w}
}

// Generate generates a Markdown scan report.
func (r *MarkdownReporter) Generate(data Data) error {
	fmt.Fprintf(r.writer, "# S3Spectre Scan Report\n\n")
	if data.Version != "" {
		fmt.Fprintf(r.writer, "**Version:** %s\n\n", data.Version)
	}
	fmt.Fprintf(r.writer, "**Scan Time:** %s  \n", data.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(r.writer, "**Repository:** %s\n\n", data.Config.RepoPath)

	fmt.Fprintf(r.writer, "## Summary\n\n")
	fmt.Fprintf(r.writer, "| Category | Count |\n|---|---|\n")
	fmt.Fprintf(r.writer, "| Total Buckets Scanned | %d |\n", data.Summary.TotalBuckets)
	fmt.Fprintf(r.writer, "| OK | %d |\n", data.Summary.OKBuckets)
	writeMarkdownCountRow(r.writer, "Missing Buckets", len(data.Summary.MissingBuckets))
	writeMarkdownCountRow(r.writer, "Unused Buckets", len(data.Summary.UnusedBuckets))
	writeMarkdownCountRow(r.writer, "Missing Prefixes", len(data.Summary.MissingPrefixes))
	writeMarkdownCountRow(r.writer, "Version Sprawl", len(data.Summary.VersionSprawl))
	writeMarkdownCountRow(r.writer, "Lifecycle Misconfig", len(data.Summary.LifecycleMisconfig))
	writeMarkdownCountRow(r.writer, "Stale Prefixes", len(data.Summary.StalePrefixes))
	writeMarkdownCountRow(r.writer, "Versioned Buckets", len(data.Summary.VersionedBuckets))
	fmt.Fprintf(r.writer, "\n")

	// Category order matches the severity ordering used by the text reporter:
	// Missing (high) -> Unused/MissingPrefix/VersionSprawl/LifecycleMisconfig
	// (medium) -> Stale (low) -> informational inventory -> OK.
	r.writeBucketSection("Missing Buckets", data.Summary.MissingBuckets, data.Buckets)
	r.writeBucketSection("Unused Buckets", data.Summary.UnusedBuckets, data.Buckets)
	r.writePrefixSection("Missing Prefixes", data.Summary.MissingPrefixes)
	r.writeBucketSection("Version Sprawl", data.Summary.VersionSprawl, data.Buckets)
	r.writeBucketSection("Lifecycle Misconfigurations", data.Summary.LifecycleMisconfig, data.Buckets)
	r.writePrefixSection("Stale Prefixes", data.Summary.StalePrefixes)
	r.writeNameListSection("Versioned Buckets", data.Summary.VersionedBuckets)

	return nil
}

// GenerateDiscovery generates a Markdown discovery report.
func (r *MarkdownReporter) GenerateDiscovery(data DiscoveryData) error {
	fmt.Fprintf(r.writer, "# S3Spectre Discovery Report\n\n")
	if data.Version != "" {
		fmt.Fprintf(r.writer, "**Version:** %s\n\n", data.Version)
	}
	fmt.Fprintf(r.writer, "**Scan Time:** %s  \n", data.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(r.writer, "**Total Regions Scanned:** %d\n\n", data.Summary.TotalRegions)

	fmt.Fprintf(r.writer, "## Summary\n\n")
	fmt.Fprintf(r.writer, "| Category | Count |\n|---|---|\n")
	fmt.Fprintf(r.writer, "| Total Buckets | %d |\n", data.Summary.TotalBuckets)
	fmt.Fprintf(r.writer, "| Healthy | %d |\n", data.Summary.HealthyBuckets)
	writeMarkdownCountRow(r.writer, "Risky", len(data.Summary.RiskyBuckets))
	writeMarkdownCountRow(r.writer, "Unused", len(data.Summary.UnusedBuckets))
	writeMarkdownCountRow(r.writer, "Inactive", len(data.Summary.InactiveBuckets))
	writeMarkdownCountRow(r.writer, "Version Sprawl", len(data.Summary.VersionSprawl))
	writeMarkdownCountRow(r.writer, "Versioned Buckets", len(data.Summary.VersionedBuckets))
	fmt.Fprintf(r.writer, "\n")

	risky := append([]string(nil), data.Summary.RiskyBuckets...)
	sortBucketsByRiskScore(risky, data.Buckets)
	r.writeDiscoverySection("Risky Buckets", risky, data.Buckets, false)
	r.writeDiscoverySection("Unused Buckets", data.Summary.UnusedBuckets, data.Buckets, true)
	r.writeDiscoverySection("Inactive Buckets", data.Summary.InactiveBuckets, data.Buckets, true)
	r.writeDiscoverySection("Version Sprawl", data.Summary.VersionSprawl, data.Buckets, true)
	r.writeNameListSection("Versioned Buckets", data.Summary.VersionedBuckets)

	return nil
}

func writeMarkdownCountRow(w io.Writer, label string, count int) {
	if count > 0 {
		fmt.Fprintf(w, "| %s | %d |\n", label, count)
	}
}

func (r *MarkdownReporter) writeBucketSection(title string, names []string, buckets map[string]*analyzer.BucketAnalysis) {
	if len(names) == 0 {
		return
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	fmt.Fprintf(r.writer, "## %s\n\n", title)
	fmt.Fprintf(r.writer, "| Bucket | Message |\n|---|---|\n")
	for _, name := range sorted {
		message := ""
		if a := buckets[name]; a != nil {
			message = escapeMarkdownTableCell(a.Message)
		}
		fmt.Fprintf(r.writer, "| %s | %s |\n", name, message)
	}
	fmt.Fprintf(r.writer, "\n")
}

func (r *MarkdownReporter) writePrefixSection(title string, prefixPaths []string) {
	if len(prefixPaths) == 0 {
		return
	}
	sorted := append([]string(nil), prefixPaths...)
	sort.Strings(sorted)
	fmt.Fprintf(r.writer, "## %s\n\n", title)
	for _, p := range sorted {
		fmt.Fprintf(r.writer, "- `%s`\n", p)
	}
	fmt.Fprintf(r.writer, "\n")
}

func (r *MarkdownReporter) writeNameListSection(title string, names []string) {
	if len(names) == 0 {
		return
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	fmt.Fprintf(r.writer, "## %s\n\n", title)
	for _, name := range sorted {
		fmt.Fprintf(r.writer, "- `%s`\n", name)
	}
	fmt.Fprintf(r.writer, "\n")
}

func (r *MarkdownReporter) writeDiscoverySection(title string, names []string, buckets map[string]*analyzer.BucketDiscovery, alphabetize bool) {
	if len(names) == 0 {
		return
	}
	ordered := names
	if alphabetize {
		ordered = append([]string(nil), names...)
		sort.Strings(ordered)
	}
	fmt.Fprintf(r.writer, "## %s\n\n", title)
	fmt.Fprintf(r.writer, "| Bucket | Region | Risk Score | Factors | Cost/mo |\n|---|---|---|---|---|\n")
	for _, name := range ordered {
		d := buckets[name]
		if d == nil {
			continue
		}
		factors := escapeMarkdownTableCell(strings.Join(d.RiskFactors, "; "))
		cost := ""
		if d.EstimatedMonthlyCostUSD > 0 {
			cost = fmt.Sprintf("$%.2f", d.EstimatedMonthlyCostUSD)
		}
		fmt.Fprintf(r.writer, "| %s | %s | %d | %s | %s |\n", name, d.Region, d.RiskScore, factors, cost)
	}
	fmt.Fprintf(r.writer, "\n")
}

func escapeMarkdownTableCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
