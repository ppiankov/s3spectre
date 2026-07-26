package report

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/remediation"
)

func TestSpectreHubReporter_Generate(t *testing.T) {
	data := Data{
		Tool:      "s3spectre",
		Version:   "0.2.0",
		Timestamp: time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC),
		Config: Config{
			AWSRegion:  "us-east-1",
			AWSProfile: "default",
		},
		Summary: analyzer.Summary{TotalBuckets: 3, OKBuckets: 1},
		Buckets: map[string]*analyzer.BucketAnalysis{
			"ok-bucket": {
				Name:   "ok-bucket",
				Status: analyzer.StatusOK,
			},
			"missing-bucket": {
				Name:    "missing-bucket",
				Status:  analyzer.StatusMissingBucket,
				Message: "referenced in code but not found in AWS",
			},
			"old-data": {
				Name:    "old-data",
				Status:  analyzer.StatusUnusedBucket,
				Message: "bucket appears unused",
				Prefixes: []analyzer.PrefixAnalysis{
					{Prefix: "archive/", Status: analyzer.StatusStalePrefix, Message: "stale prefix"},
				},
			},
		},
	}

	var buf bytes.Buffer
	r := NewSpectreHubReporter(&buf)
	if err := r.Generate(data); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var envelope spectreEnvelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if envelope.Schema != "spectre/v1" {
		t.Errorf("schema = %q, want spectre/v1", envelope.Schema)
	}
	if envelope.Tool != "s3spectre" {
		t.Errorf("tool = %q, want s3spectre", envelope.Tool)
	}
	if envelope.Target.Type != "s3" {
		t.Errorf("target.type = %q, want s3", envelope.Target.Type)
	}
	// OK bucket excluded, 2 bucket findings + 1 prefix finding = 3
	if envelope.Summary.Total != 3 {
		t.Errorf("summary.total = %d, want 3", envelope.Summary.Total)
	}
	if envelope.Summary.High != 1 {
		t.Errorf("summary.high = %d, want 1 (MISSING_BUCKET)", envelope.Summary.High)
	}

	// Verify findings contain expected IDs
	ids := make(map[string]bool)
	for _, f := range envelope.Findings {
		ids[f.ID] = true
	}
	if !ids["MISSING_BUCKET"] {
		t.Error("expected MISSING_BUCKET finding")
	}
	if !ids["UNUSED_BUCKET"] {
		t.Error("expected UNUSED_BUCKET finding")
	}
	if !ids["STALE_PREFIX"] {
		t.Error("expected STALE_PREFIX finding")
	}
}

func TestSpectreHubReporter_GenerateDiscovery(t *testing.T) {
	data := DiscoveryData{
		Tool:      "s3spectre",
		Version:   "0.2.0",
		Timestamp: time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC),
		Config: DiscoveryConfig{
			AWSProfile: "prod",
		},
		Summary: analyzer.DiscoverySummary{TotalBuckets: 2, HealthyBuckets: 1},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"healthy-bucket": {
				Name:   "healthy-bucket",
				Status: analyzer.StatusOK,
			},
			"risky-bucket": {
				Name:            "risky-bucket",
				Region:          "us-east-1",
				Status:          analyzer.StatusRisky,
				RiskScore:       90,
				RiskFactors:     []string{"Public access enabled", "No encryption"},
				Recommendations: []string{"Restrict public access"},
			},
		},
	}

	var buf bytes.Buffer
	r := NewSpectreHubReporter(&buf)
	if err := r.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery: %v", err)
	}

	var envelope spectreEnvelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if envelope.Schema != "spectre/v1" {
		t.Errorf("schema = %q, want spectre/v1", envelope.Schema)
	}
	// OK excluded, only risky bucket
	if len(envelope.Findings) != 1 {
		t.Fatalf("findings count = %d, want 1", len(envelope.Findings))
	}
	if envelope.Findings[0].ID != "RISKY" {
		t.Errorf("findings[0].id = %q, want RISKY", envelope.Findings[0].ID)
	}
	if envelope.Findings[0].Severity != "high" {
		t.Errorf("findings[0].severity = %q, want high (risk_score=90)", envelope.Findings[0].Severity)
	}
	if envelope.Summary.Total != 1 || envelope.Summary.High != 1 {
		t.Errorf("summary = total=%d high=%d, want 1/1", envelope.Summary.Total, envelope.Summary.High)
	}
}

// TestSpectreHubReporter_GenerateDiscovery_LifecycleSuggestionAndStorageCost
// guards WO-43/WO-44 wiring into the SpectreHub metadata envelope: a
// lifecycle suggestion and an inactive-bucket storage cost estimate must
// both surface in the finding's metadata.
func TestSpectreHubReporter_GenerateDiscovery_LifecycleSuggestionAndStorageCost(t *testing.T) {
	data := DiscoveryData{
		Tool:    "s3spectre",
		Version: "0.4.0",
		Summary: analyzer.DiscoverySummary{TotalBuckets: 2},
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
			"inactive-bucket": {
				Name:                    "inactive-bucket",
				Status:                  analyzer.StatusInactive,
				RiskScore:               100,
				EstimatedStorageCostUSD: 12.5,
			},
		},
	}

	var buf bytes.Buffer
	r := NewSpectreHubReporter(&buf)
	if err := r.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery: %v", err)
	}

	var envelope spectreEnvelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	byLocation := map[string]spectreFinding{}
	for _, f := range envelope.Findings {
		byLocation[f.Location] = f
	}

	sprawl, ok := byLocation["sprawling-bucket"]
	if !ok {
		t.Fatal("expected a finding for sprawling-bucket")
	}
	if sprawl.Metadata["lifecycle_policy_suggestion"] == nil {
		t.Error("expected lifecycle_policy_suggestion in metadata for a version-sprawl finding with a suggestion attached")
	}

	inactive, ok := byLocation["inactive-bucket"]
	if !ok {
		t.Fatal("expected a finding for inactive-bucket")
	}
	cost, ok := inactive.Metadata["estimated_monthly_cost_usd"].(float64)
	if !ok || cost != 12.5 {
		t.Errorf("expected estimated_monthly_cost_usd=12.5 (from EstimatedStorageCostUSD via CostUSD()), got %v", inactive.Metadata["estimated_monthly_cost_usd"])
	}
}

func TestSpectreHubReporter_GenerateDiscovery_TotalEstimatedCostUSD(t *testing.T) {
	data := DiscoveryData{
		Tool:    "s3spectre",
		Version: "0.5.0",
		Summary: analyzer.DiscoverySummary{TotalBuckets: 1, TotalEstimatedCostUSD: 11.99},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"stale": {Name: "stale", Status: analyzer.StatusInactive, RiskScore: 100},
		},
	}

	var buf bytes.Buffer
	r := NewSpectreHubReporter(&buf)
	if err := r.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery: %v", err)
	}

	var envelope spectreEnvelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if envelope.Summary.TotalEstimatedCostUSD != 11.99 {
		t.Errorf("summary.total_estimated_cost_usd = %v, want 11.99", envelope.Summary.TotalEstimatedCostUSD)
	}
}

// TestSpectreHubReporter_GenerateDiscovery_TagRollup guards format parity:
// --group-by-tag data must reach the spectrehub envelope the same way it
// reaches text/markdown/plain-JSON output, not be silently dropped.
func TestSpectreHubReporter_GenerateDiscovery_TagRollup(t *testing.T) {
	data := DiscoveryData{
		Tool:    "s3spectre",
		Version: "0.5.0",
		Summary: analyzer.DiscoverySummary{
			TotalBuckets: 1,
			TagRollup: map[string]*analyzer.TagGroupSummary{
				"backend": {BucketCount: 1, RiskScore: 50},
			},
		},
	}

	var buf bytes.Buffer
	r := NewSpectreHubReporter(&buf)
	if err := r.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery: %v", err)
	}

	var envelope spectreEnvelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	backend := envelope.Summary.TagRollup["backend"]
	if backend == nil || backend.BucketCount != 1 || backend.RiskScore != 50 {
		t.Errorf("expected tag_rollup.backend in spectrehub envelope, got %+v", envelope.Summary.TagRollup)
	}
}

func TestSpectreHubReporter_EmptyFindings(t *testing.T) {
	data := Data{
		Tool:      "s3spectre",
		Version:   "0.2.0",
		Timestamp: time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC),
		Buckets:   map[string]*analyzer.BucketAnalysis{},
	}

	var buf bytes.Buffer
	r := NewSpectreHubReporter(&buf)
	if err := r.Generate(data); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var envelope spectreEnvelope
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if envelope.Findings == nil {
		t.Fatal("findings should be empty array, not null")
	}
	if len(envelope.Findings) != 0 {
		t.Errorf("findings count = %d, want 0", len(envelope.Findings))
	}
}

func TestHashRegion(t *testing.T) {
	h1 := HashRegion("us-east-1", "default")
	h2 := HashRegion("us-east-1", "default")
	if h1 != h2 {
		t.Errorf("same inputs should produce same hash: %s != %s", h1, h2)
	}

	h3 := HashRegion("eu-west-1", "default")
	if h1 == h3 {
		t.Errorf("different regions should produce different hashes")
	}

	if len(h1) < 10 || h1[:7] != "sha256:" {
		t.Errorf("hash should start with sha256:, got %q", h1)
	}
}
