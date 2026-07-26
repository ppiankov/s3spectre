package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/s3"
)

func setNoColor(t *testing.T) {
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() {
		color.NoColor = prev
	})
}

func TestTextReporter_EmptyInput(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		Config: Config{
			RepoPath:           "/repo",
			StaleThresholdDays: 90,
		},
		Summary: analyzer.Summary{},
		Buckets: map[string]*analyzer.BucketAnalysis{},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "S3Spectre") {
		t.Fatalf("expected report header, got: %s", out)
	}
	if !strings.Contains(out, "Summary") {
		t.Fatalf("expected summary section, got: %s", out)
	}
	if !strings.Contains(out, "Total Buckets Scanned: 0") {
		t.Fatalf("expected total buckets line, got: %s", out)
	}
	if strings.Contains(out, "Missing Buckets") {
		t.Fatalf("did not expect missing buckets section, got: %s", out)
	}
}

func TestTextReporter_OutputFormat(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	summary := analyzer.Summary{
		TotalBuckets:       5,
		OKBuckets:          1,
		MissingBuckets:     []string{"missing-bucket"},
		UnusedBuckets:      []string{"unused-bucket"},
		MissingPrefixes:    []string{"ok-bucket/missing-prefix"},
		StalePrefixes:      []string{"ok-bucket/stale-prefix"},
		VersionSprawl:      []string{"sprawl-bucket"},
		LifecycleMisconfig: []string{"lifecycle-bucket"},
	}

	buckets := map[string]*analyzer.BucketAnalysis{
		"missing-bucket": {
			Name:    "missing-bucket",
			Status:  analyzer.StatusMissingBucket,
			Message: "not found",
		},
		"unused-bucket": {
			Name:    "unused-bucket",
			Status:  analyzer.StatusUnusedBucket,
			Message: "unused",
			UnusedScore: &analyzer.UnusedScore{
				Total:   200,
				Reasons: []string{"no references", "empty"},
			},
		},
		"sprawl-bucket": {
			Name:    "sprawl-bucket",
			Status:  analyzer.StatusVersionSprawl,
			Message: "too many versions",
		},
		"lifecycle-bucket": {
			Name:    "lifecycle-bucket",
			Status:  analyzer.StatusLifecycleMisconfig,
			Message: "missing rule",
		},
		"ok-bucket": {
			Name:   "ok-bucket",
			Status: analyzer.StatusOK,
		},
	}

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC),
		Config: Config{
			RepoPath:           "/repo",
			AWSProfile:         "default",
			AWSRegion:          "us-east-1",
			StaleThresholdDays: 90,
		},
		Summary: summary,
		Buckets: buckets,
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	checks := []string{
		"S3Spectre",
		"Repository: /repo",
		"AWS Profile: default",
		"AWS Region: us-east-1",
		"Summary",
		"Missing Buckets",
		"[MISSING_BUCKET]",
		"Unused Buckets",
		"Reasons:",
		"Missing Prefixes",
		"Stale Prefixes",
		"Version Sprawl",
		"Lifecycle Misconfigurations",
		"OK Buckets: 1",
	}

	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected output to contain %q, got: %s", check, out)
		}
	}
}

func TestTextReporter_LargeDiscoveryOutput(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	buckets := make(map[string]*analyzer.BucketDiscovery)
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("healthy-%02d", i)
		buckets[name] = &analyzer.BucketDiscovery{
			Name:   name,
			Region: "us-east-1",
			Status: analyzer.StatusOK,
		}
	}

	buckets["sprawl-bucket"] = &analyzer.BucketDiscovery{
		Name:   "sprawl-bucket",
		Region: "us-east-1",
		Status: analyzer.StatusVersionSprawl,
		BucketInfo: &s3.BucketInfo{
			TotalVersionSize: 5 * 1024 * 1024,
			TotalSize:        1 * 1024 * 1024,
			VersionCount:     42,
		},
		RiskFactors: []string{"Versioning enabled without lifecycle rules"},
	}

	data := DiscoveryData{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC),
		Config: DiscoveryConfig{
			AllRegions: true,
		},
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:   13,
			HealthyBuckets: 12,
			VersionSprawl:  []string{"sprawl-bucket"},
			TotalRegions:   1,
		},
		Buckets: buckets,
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Healthy Buckets: 12") {
		t.Fatalf("expected healthy buckets summary, got: %s", out)
	}
	if !strings.Contains(out, "... and 2 more") {
		t.Fatalf("expected truncation line, got: %s", out)
	}
	if !strings.Contains(out, "Total Size (all versions):") {
		t.Fatalf("expected version size details, got: %s", out)
	}
	if !strings.Contains(out, "Version Overhead:") {
		t.Fatalf("expected version overhead details, got: %s", out)
	}
}

// TestTextReporter_CategoryOrderBySeverity guards against categories printing
// in a fixed order that doesn't track severity (e.g. low-severity Stale
// Prefixes appearing ahead of medium-severity categories).
func TestTextReporter_CategoryOrderBySeverity(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	summary := analyzer.Summary{
		TotalBuckets:       2,
		StalePrefixes:      []string{"bucket/stale-prefix"},
		LifecycleMisconfig: []string{"lifecycle-bucket"},
	}
	buckets := map[string]*analyzer.BucketAnalysis{
		"lifecycle-bucket": {Name: "lifecycle-bucket", Status: analyzer.StatusLifecycleMisconfig},
	}

	data := Data{
		Tool:    "s3spectre",
		Version: "0.1.0",
		Config:  Config{RepoPath: "/repo"},
		Summary: summary,
		Buckets: buckets,
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	// Use the finding markers, not the section headings, since "Stale Prefixes"
	// also appears (identically) in the summary block above the findings.
	lifecycleIdx := strings.Index(out, "[LIFECYCLE_MISCONFIG]")
	staleIdx := strings.Index(out, "[STALE_PREFIX]")
	if lifecycleIdx == -1 || staleIdx == -1 {
		t.Fatalf("expected both sections present, got: %s", out)
	}
	if lifecycleIdx > staleIdx {
		t.Fatalf("expected medium-severity Lifecycle Misconfigurations before low-severity Stale Prefixes, got: %s", out)
	}
}

// TestTextReporter_RiskyBucketsSortedByRiskScore guards against RiskyBuckets
// rendering in alphabetical order instead of risk-score-descending, which
// would bury the most dangerous bucket among lower-risk noise.
func TestTextReporter_RiskyBucketsSortedByRiskScore(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	buckets := map[string]*analyzer.BucketDiscovery{
		"aaa-low-risk":  {Name: "aaa-low-risk", Status: analyzer.StatusRisky, RiskScore: 40},
		"zzz-high-risk": {Name: "zzz-high-risk", Status: analyzer.StatusRisky, RiskScore: 95},
	}

	data := DiscoveryData{
		Tool:    "s3spectre",
		Version: "0.1.0",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets: 2,
			RiskyBuckets: []string{"aaa-low-risk", "zzz-high-risk"},
		},
		Buckets: buckets,
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	highIdx := strings.Index(out, "zzz-high-risk")
	lowIdx := strings.Index(out, "aaa-low-risk")
	if highIdx == -1 || lowIdx == -1 {
		t.Fatalf("expected both buckets present, got: %s", out)
	}
	if highIdx > lowIdx {
		t.Fatalf("expected higher risk-score bucket (95) before lower (40) despite alphabetical order, got: %s", out)
	}
}

// TestTextReporter_VersionedBucketsInventory guards against the new
// informational versioned-bucket section failing to render for either scan or
// discover output.
func TestTextReporter_VersionedBucketsInventory_Scan(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := Data{
		Tool:   "s3spectre",
		Config: Config{RepoPath: "/repo"},
		Summary: analyzer.Summary{
			TotalBuckets:     1,
			VersionedBuckets: []string{"well-managed"},
		},
		Buckets: map[string]*analyzer.BucketAnalysis{
			"well-managed": {Name: "well-managed", Status: analyzer.StatusOK},
		},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Versioned Buckets") || !strings.Contains(out, "well-managed") {
		t.Fatalf("expected versioned-buckets section with well-managed, got: %s", out)
	}
}

func TestTextReporter_VersionedBucketsInventory_Discover(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:     1,
			VersionedBuckets: []string{"well-managed"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"well-managed": {Name: "well-managed", Status: analyzer.StatusOK, Region: "us-east-1"},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Versioned Buckets") || !strings.Contains(out, "well-managed") {
		t.Fatalf("expected versioned-buckets section with well-managed, got: %s", out)
	}
}

func TestTextReporter_EstimatedCost_ShownWhenPresent(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:  1,
			VersionSprawl: []string{"sprawling"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"sprawling": {
				Name:                    "sprawling",
				Status:                  analyzer.StatusVersionSprawl,
				Region:                  "us-east-1",
				EstimatedMonthlyCostUSD: 1.23,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Estimated Cost") || !strings.Contains(out, "$1.23/month") {
		t.Fatalf("expected estimated cost line, got: %s", out)
	}
}

func TestTextReporter_EstimatedCost_OmittedWhenZero(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:  1,
			VersionSprawl: []string{"sprawling"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"sprawling": {Name: "sprawling", Status: analyzer.StatusVersionSprawl, Region: "us-east-1"},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Estimated Cost") {
		t.Fatalf("expected no estimated-cost line when EstimateCost is off, got: %s", out)
	}
}
