package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/remediation"
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

func TestTextReporter_PublicBucketsInventory(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:  1,
			PublicBuckets: []string{"allowlisted-public"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"allowlisted-public": {Name: "allowlisted-public", Status: analyzer.StatusOK, Region: "us-east-1"},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Public Buckets") || !strings.Contains(out, "allowlisted-public") {
		t.Fatalf("expected Public Buckets inventory section, got: %s", out)
	}
}

func TestTextReporter_LifecycleSuggestion_ShownWhenPresent(t *testing.T) {
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
				Name:   "sprawling",
				Status: analyzer.StatusVersionSprawl,
				Region: "us-east-1",
				LifecyclePolicySuggestion: &remediation.LifecyclePolicySuggestion{
					JSON:      `{"Rules":[]}`,
					Terraform: `resource "aws_s3_bucket_lifecycle_configuration" "sprawling" {}`,
				},
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Suggested lifecycle rule") {
		t.Fatalf("expected lifecycle suggestion block, got: %s", out)
	}
}

func TestTextReporter_EstimatedStorageCost_ShownForUnusedBucket(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:  1,
			UnusedBuckets: []string{"empty-and-old"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"empty-and-old": {
				Name:                    "empty-and-old",
				Status:                  analyzer.StatusUnusedBucket,
				Region:                  "us-east-1",
				EstimatedStorageCostUSD: 2.10,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Estimated Storage Cost") || !strings.Contains(out, "$2.10/month") {
		t.Fatalf("expected estimated storage cost line for unused bucket, got: %s", out)
	}
}

func TestTextReporter_EstimatedStorageCost_ShownForInactiveBucket(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:    1,
			InactiveBuckets: []string{"stale"},
		},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"stale": {
				Name:                    "stale",
				Status:                  analyzer.StatusInactive,
				Region:                  "us-east-1",
				EstimatedStorageCostUSD: 3.45,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Estimated Storage Cost") || !strings.Contains(out, "$3.45/month") {
		t.Fatalf("expected estimated storage cost line for inactive bucket, got: %s", out)
	}
}

func TestTextReporter_TotalEstimatedCost_ShownWhenPresent(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets:          1,
			TotalEstimatedCostUSD: 11.99,
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Total Estimated Cost") || !strings.Contains(out, "$11.99/month") {
		t.Fatalf("expected total estimated cost line, got: %s", out)
	}
}

func TestTextReporter_TotalEstimatedCost_OmittedWhenZero(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool:    "s3spectre",
		Summary: analyzer.DiscoverySummary{TotalBuckets: 1},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	if strings.Contains(buf.String(), "Total Estimated Cost") {
		t.Fatalf("expected no total-cost line when zero, got: %s", buf.String())
	}
}

func TestTextReporter_TagRollup_SortedByRiskScoreDescending(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets: 3,
			TagRollup: map[string]*analyzer.TagGroupSummary{
				"frontend": {BucketCount: 1, RiskScore: 20},
				"backend":  {BucketCount: 2, RiskScore: 150, UnusedCount: 1, RiskyCount: 1},
				"untagged": {BucketCount: 1, RiskScore: 60},
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Rollup by tag") {
		t.Fatalf("expected a tag-rollup section, got: %s", out)
	}
	backendIdx := strings.Index(out, "backend:")
	untaggedIdx := strings.Index(out, "untagged:")
	frontendIdx := strings.Index(out, "frontend:")
	if backendIdx == -1 || untaggedIdx == -1 || frontendIdx == -1 {
		t.Fatalf("expected all three tag groups present, got: %s", out)
	}
	if !(backendIdx < untaggedIdx && untaggedIdx < frontendIdx) {
		t.Fatalf("expected rollup sorted by descending risk score (backend > untagged > frontend), got order in: %s", out)
	}
}

func TestTextReporter_TagRollup_OmittedWhenEmpty(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{Tool: "s3spectre", Summary: analyzer.DiscoverySummary{TotalBuckets: 1}}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	if strings.Contains(buf.String(), "Rollup by tag") {
		t.Fatalf("expected no rollup section when TagRollup is empty, got: %s", buf.String())
	}
}

// TestTextReporter_TagRollup_TiedRiskScoreBreaksAlphabetically guards the
// sort comparator's tiebreaker: when two tag groups have the exact same
// risk score, order must fall back to alphabetical by tag value, not be
// left to map iteration's nondeterministic order.
func TestTextReporter_TagRollup_TiedRiskScoreBreaksAlphabetically(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets: 2,
			TagRollup: map[string]*analyzer.TagGroupSummary{
				"zeta":  {BucketCount: 1, RiskScore: 50},
				"alpha": {BucketCount: 1, RiskScore: 50},
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	alphaIdx := strings.Index(out, "alpha:")
	zetaIdx := strings.Index(out, "zeta:")
	if alphaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("expected both tag groups present, got: %s", out)
	}
	if alphaIdx > zetaIdx {
		t.Fatalf("expected alphabetical tiebreak (alpha before zeta) for tied risk scores, got order in: %s", out)
	}
}

func TestTextReporter_TagRollup_ShowsAverageRiskScore(t *testing.T) {
	setNoColor(t)
	var buf bytes.Buffer
	reporter := NewTextReporter(&buf)

	data := DiscoveryData{
		Tool: "s3spectre",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets: 2,
			TagRollup: map[string]*analyzer.TagGroupSummary{
				"backend": {BucketCount: 2, RiskScore: 40, AverageRiskScore: 20},
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "avg 20.0/bucket") {
		t.Fatalf("expected average risk score in the rollup line, got: %s", out)
	}
}
