package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

func TestJSONReporter_Generate(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewJSONReporter(&buf)

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC),
		Config: Config{
			RepoPath:           "/repo",
			AWSProfile:         "default",
			AWSRegion:          "us-east-1",
			StaleThresholdDays: 30,
		},
		Summary: analyzer.Summary{
			TotalBuckets: 2,
			OKBuckets:    1,
			MissingBuckets: []string{
				"missing-bucket",
			},
		},
		Buckets: map[string]*analyzer.BucketAnalysis{
			"missing-bucket": {
				Name:   "missing-bucket",
				Status: analyzer.StatusMissingBucket,
			},
			"ok-bucket": {
				Name:   "ok-bucket",
				Status: analyzer.StatusOK,
			},
		},
		References: []scanner.Reference{
			{
				Bucket:  "ok-bucket",
				Prefix:  "data",
				File:    "main.tf",
				Line:    10,
				Context: "terraform",
			},
		},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Fatalf("output is not valid JSON: %s", buf.String())
	}

	var decoded Data
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if decoded.Tool != data.Tool {
		t.Fatalf("expected tool %q, got %q", data.Tool, decoded.Tool)
	}
	if decoded.Summary.TotalBuckets != 2 {
		t.Fatalf("expected total buckets 2, got %d", decoded.Summary.TotalBuckets)
	}
	if len(decoded.References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(decoded.References))
	}
}

func TestJSONReporter_TimestampUTC(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewJSONReporter(&buf)

	loc := time.FixedZone("PST", -8*60*60)
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, loc)

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: ts,
		Config: Config{
			RepoPath:           "/repo",
			StaleThresholdDays: 30,
		},
		Summary: analyzer.Summary{},
		Buckets: map[string]*analyzer.BucketAnalysis{},
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	var decoded struct {
		Tool      string `json:"tool"`
		Version   string `json:"version"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	expected := ts.UTC().Format(time.RFC3339)
	if decoded.Tool != data.Tool {
		t.Fatalf("expected tool %q, got %q", data.Tool, decoded.Tool)
	}
	if decoded.Version != data.Version {
		t.Fatalf("expected version %q, got %q", data.Version, decoded.Version)
	}
	if decoded.Timestamp != expected {
		t.Fatalf("expected timestamp %q, got %q", expected, decoded.Timestamp)
	}
}

func TestJSONReporter_EmptyDiscovery(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewJSONReporter(&buf)

	data := DiscoveryData{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
		Config: DiscoveryConfig{
			AllRegions: false,
			Regions:    nil,
		},
		Summary: analyzer.DiscoverySummary{},
		Buckets: map[string]*analyzer.BucketDiscovery{},
	}

	if err := reporter.GenerateDiscovery(data); err != nil {
		t.Fatalf("GenerateDiscovery failed: %v", err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Fatalf("output is not valid JSON: %s", buf.String())
	}

	var decoded DiscoveryData
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if decoded.Summary.TotalBuckets != 0 {
		t.Fatalf("expected total buckets 0, got %d", decoded.Summary.TotalBuckets)
	}
}

func TestJSONReporter_LargeInput(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewJSONReporter(&buf)

	buckets := make(map[string]*analyzer.BucketAnalysis)
	refs := make([]scanner.Reference, 0, 50)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("bucket-%02d", i)
		buckets[name] = &analyzer.BucketAnalysis{
			Name:   name,
			Status: analyzer.StatusOK,
		}
		refs = append(refs, scanner.Reference{
			Bucket:  name,
			Prefix:  "data",
			File:    "file.tf",
			Line:    i + 1,
			Context: "terraform",
		})
	}

	data := Data{
		Tool:      "s3spectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC),
		Config: Config{
			RepoPath:           "/repo",
			StaleThresholdDays: 60,
		},
		Summary: analyzer.Summary{
			TotalBuckets: 50,
			OKBuckets:    50,
		},
		Buckets:    buckets,
		References: refs,
	}

	if err := reporter.Generate(data); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Fatalf("output is not valid JSON: %s", buf.String())
	}

	var decoded Data
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if len(decoded.Buckets) != 50 {
		t.Fatalf("expected 50 buckets, got %d", len(decoded.Buckets))
	}
	if len(decoded.References) != 50 {
		t.Fatalf("expected 50 references, got %d", len(decoded.References))
	}
}
