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
