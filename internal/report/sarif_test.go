package report

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/s3"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

type sarifResultOutput struct {
	RuleID  string `json:"ruleId"`
	Level   string `json:"level"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
	Locations []struct {
		PhysicalLocation *struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region *struct {
				StartLine int `json:"startLine"`
			} `json:"region"`
		} `json:"physicalLocation"`
	} `json:"locations"`
}

type sarifOutput struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []struct {
		Tool struct {
			Driver struct {
				Name  string `json:"name"`
				Rules []struct {
					ID string `json:"id"`
				} `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []sarifResultOutput `json:"results"`
	} `json:"runs"`
}

func TestSARIFReporter_Generate(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewSARIFReporter(&buf)

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.2.0",
		Timestamp: time.Date(2024, 7, 8, 9, 10, 11, 0, time.UTC),
		Config: Config{
			RepoPath:           "/repo",
			StaleThresholdDays: 90,
		},
		Summary: analyzer.Summary{},
		Buckets: map[string]*analyzer.BucketAnalysis{
			"missing-bucket": {
				Name:    "missing-bucket",
				Status:  analyzer.StatusMissingBucket,
				Message: "Bucket referenced in code but does not exist in AWS",
			},
			"lifecycle-bucket": {
				Name:    "lifecycle-bucket",
				Status:  analyzer.StatusLifecycleMisconfig,
				Message: "Bucket has no lifecycle rules but contains many objects",
			},
			"ok-bucket": {
				Name:   "ok-bucket",
				Status: analyzer.StatusOK,
				Prefixes: []analyzer.PrefixAnalysis{
					{
						Prefix:  "missing",
						Status:  analyzer.StatusMissingPrefix,
						Message: "Prefix referenced in code but no objects found",
					},
					{
						Prefix:  "stale",
						Status:  analyzer.StatusStalePrefix,
						Message: "No modifications for 120 days (threshold: 90)",
					},
				},
			},
		},
		References: []scanner.Reference{
			{Bucket: "missing-bucket", File: "main.tf", Line: 10},
			{Bucket: "ok-bucket", Prefix: "missing", File: "main.tf", Line: 20},
			{Bucket: "ok-bucket", Prefix: "stale", File: "main.tf", Line: 25},
			{Bucket: "lifecycle-bucket", File: "main.tf", Line: 30},
		},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatalf("output is not valid JSON: %s", buf.String())
	}

	var decoded sarifOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if decoded.Version != sarifVersion {
		t.Fatalf("expected sarif version %q, got %q", sarifVersion, decoded.Version)
	}
	if decoded.Schema != sarifSchema {
		t.Fatalf("expected schema %q, got %q", sarifSchema, decoded.Schema)
	}
	if len(decoded.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(decoded.Runs))
	}
	if decoded.Runs[0].Tool.Driver.Name != data.Tool {
		t.Fatalf("expected tool name %q, got %q", data.Tool, decoded.Runs[0].Tool.Driver.Name)
	}

	missing, ok := findResult(decoded.Runs[0].Results, sarifRuleMissingBucket)
	if !ok {
		t.Fatalf("missing result for %s", sarifRuleMissingBucket)
	}
	if missing.Level != "warning" {
		t.Fatalf("expected missing bucket level warning, got %q", missing.Level)
	}
	if len(missing.Locations) == 0 || missing.Locations[0].PhysicalLocation == nil {
		t.Fatalf("expected missing bucket to include a location")
	}
	if missing.Locations[0].PhysicalLocation.ArtifactLocation.URI != "main.tf" {
		t.Fatalf("expected location uri main.tf, got %q", missing.Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
	if missing.Locations[0].PhysicalLocation.Region == nil || missing.Locations[0].PhysicalLocation.Region.StartLine != 10 {
		t.Fatalf("expected location line 10, got %+v", missing.Locations[0].PhysicalLocation.Region)
	}

	lifecycle, ok := findResult(decoded.Runs[0].Results, sarifRuleLifecycleGap)
	if !ok {
		t.Fatalf("missing result for %s", sarifRuleLifecycleGap)
	}
	if lifecycle.Level != "note" {
		t.Fatalf("expected lifecycle gap level note, got %q", lifecycle.Level)
	}
}

func TestSARIFReporter_GenerateDiscovery(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewSARIFReporter(&buf)

	bucketInfo := &s3.BucketInfo{
		Name:         "public-bucket",
		Region:       "us-east-1",
		PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
		Encryption:   &s3.EncryptionInfo{Enabled: false},
	}

	data := DiscoveryData{
		Tool:      "s3spectre",
		Version:   "0.2.0",
		Timestamp: time.Date(2024, 7, 8, 9, 10, 11, 0, time.UTC),
		Config: DiscoveryConfig{
			CheckEncryption:   true,
			CheckPublicAccess: true,
		},
		Summary: analyzer.DiscoverySummary{},
		Buckets: map[string]*analyzer.BucketDiscovery{
			"public-bucket": {
				Name:       "public-bucket",
				Region:     "us-east-1",
				Status:     analyzer.StatusRisky,
				RiskScore:  120,
				BucketInfo: bucketInfo,
			},
		},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatalf("output is not valid JSON: %s", buf.String())
	}

	var decoded sarifOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if len(decoded.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(decoded.Runs))
	}

	publicResult, ok := findResult(decoded.Runs[0].Results, sarifRulePublicBucket)
	if !ok {
		t.Fatalf("missing result for %s", sarifRulePublicBucket)
	}
	if publicResult.Level != "error" {
		t.Fatalf("expected public bucket level error, got %q", publicResult.Level)
	}

	encryptionResult, ok := findResult(decoded.Runs[0].Results, sarifRuleNoEncryption)
	if !ok {
		t.Fatalf("missing result for %s", sarifRuleNoEncryption)
	}
	if encryptionResult.Level != "warning" {
		t.Fatalf("expected no encryption level warning, got %q", encryptionResult.Level)
	}
}

func findResult(results []sarifResultOutput, ruleID string) (sarifResultOutput, bool) {
	for _, result := range results {
		if result.RuleID == ruleID {
			return result, true
		}
	}
	return sarifResultOutput{}, false
}
