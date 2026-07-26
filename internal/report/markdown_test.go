package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/remediation"
)

func TestMarkdownReporter_Generate_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewMarkdownReporter(&buf)

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.3.0",
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Config:    Config{RepoPath: "/repo"},
		Summary:   analyzer.Summary{},
		Buckets:   map[string]*analyzer.BucketAnalysis{},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# S3Spectre Scan Report") {
		t.Fatalf("expected a Markdown heading, got: %s", out)
	}
	if !strings.Contains(out, "| Total Buckets Scanned | 0 |") {
		t.Fatalf("expected summary table with zero counts, got: %s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI escape codes in Markdown output, got: %s", out)
	}
}

func TestMarkdownReporter_Generate_MixedSeverityFindings(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewMarkdownReporter(&buf)

	data := Data{
		Tool:    "s3spectre",
		Version: "0.3.0",
		Config:  Config{RepoPath: "/repo"},
		Summary: analyzer.Summary{
			TotalBuckets:   2,
			MissingBuckets: []string{"gone-bucket"},
			StalePrefixes:  []string{"gone-bucket/old-prefix"},
		},
		Buckets: map[string]*analyzer.BucketAnalysis{
			"gone-bucket": {Name: "gone-bucket", Status: analyzer.StatusMissingBucket, Message: "does not exist"},
		},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## Missing Buckets") || !strings.Contains(out, "gone-bucket") {
		t.Fatalf("expected Missing Buckets section, got: %s", out)
	}
	if !strings.Contains(out, "## Stale Prefixes") || !strings.Contains(out, "old-prefix") {
		t.Fatalf("expected Stale Prefixes section, got: %s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI escape codes, got: %s", out)
	}
}

func TestMarkdownReporter_GenerateDiscovery_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewMarkdownReporter(&buf)

	data := DiscoveryData{
		Tool:    "s3spectre",
		Version: "0.3.0",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:     2,
			HealthyBuckets:   1,
			RiskyBuckets:     []string{"risky-one"},
			VersionedBuckets: []string{"risky-one"},
			TotalRegions:     1,
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"risky-one": {
				Name:                    "risky-one",
				Status:                  analyzer.StatusRisky,
				Region:                  "us-east-1",
				RiskScore:               85,
				RiskFactors:             []string{"Public access enabled"},
				EstimatedMonthlyCostUSD: 4.5,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "# S3Spectre Discovery Report") {
		t.Fatalf("expected discovery heading, got: %s", out)
	}
	if !strings.Contains(out, "## Risky Buckets") || !strings.Contains(out, "risky-one") {
		t.Fatalf("expected Risky Buckets section, got: %s", out)
	}
	if !strings.Contains(out, "$4.50") {
		t.Fatalf("expected estimated cost in the table, got: %s", out)
	}
	if !strings.Contains(out, "## Versioned Buckets") {
		t.Fatalf("expected Versioned Buckets section, got: %s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI escape codes, got: %s", out)
	}
}

func TestMarkdownReporter_GenerateDiscovery_PublicBucketsAndLifecycleSuggestion(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewMarkdownReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:  2,
			VersionSprawl: []string{"sprawling-bucket"},
			PublicBuckets: []string{"allowlisted-public"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"sprawling-bucket": {
				Name:      "sprawling-bucket",
				Status:    analyzer.StatusVersionSprawl,
				RiskScore: 30,
				LifecyclePolicySuggestion: &remediation.LifecyclePolicySuggestion{
					JSON:      `{"Rules":[]}`,
					Terraform: `resource "aws_s3_bucket_lifecycle_configuration" "sprawling_bucket" {}`,
				},
			},
			"allowlisted-public": {
				Name:      "allowlisted-public",
				Status:    analyzer.StatusOK,
				RiskScore: 30,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## Public Buckets") || !strings.Contains(out, "allowlisted-public") {
		t.Fatalf("expected Public Buckets inventory section, got: %s", out)
	}
	if !strings.Contains(out, "Suggested lifecycle rules") {
		t.Fatalf("expected lifecycle suggestion section header, got: %s", out)
	}
	if !strings.Contains(out, "```json") || !strings.Contains(out, "```hcl") {
		t.Fatalf("expected fenced JSON and HCL code blocks, got: %s", out)
	}
}

// TestMarkdownReporter_GenerateDiscovery_StorageCostFallbackInTable guards
// against the "Cost/mo" column only ever showing version-sprawl overhead
// cost; an Inactive/Unused bucket's EstimatedStorageCostUSD must also render
// via the CostUSD() fallback.
func TestMarkdownReporter_GenerateDiscovery_StorageCostFallbackInTable(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewMarkdownReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:    1,
			InactiveBuckets: []string{"stale-archive"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"stale-archive": {
				Name:                    "stale-archive",
				Status:                  analyzer.StatusInactive,
				Region:                  "us-east-1",
				RiskScore:               100,
				EstimatedStorageCostUSD: 7.89,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "$7.89") {
		t.Fatalf("expected storage cost to render in the Inactive Buckets table via CostUSD() fallback, got: %s", out)
	}
}

func TestMarkdownReporter_EscapesPipeInTableCells(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewMarkdownReporter(&buf)

	data := Data{
		Tool: "s3spectre",
		Summary: analyzer.Summary{
			TotalBuckets:   1,
			MissingBuckets: []string{"weird-bucket"},
		},
		Buckets: map[string]*analyzer.BucketAnalysis{
			"weird-bucket": {Name: "weird-bucket", Status: analyzer.StatusMissingBucket, Message: "contains | a pipe"},
		},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `contains \| a pipe`) {
		t.Fatalf("expected escaped pipe character in table cell, got: %s", out)
	}
}
