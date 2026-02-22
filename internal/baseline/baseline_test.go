package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/report"
)

func TestFlattenScanFindings(t *testing.T) {
	data := report.Data{
		Buckets: map[string]*analyzer.BucketAnalysis{
			"bucket-a": {Status: analyzer.StatusMissingBucket},
			"bucket-b": {
				Status: analyzer.StatusOK,
				Prefixes: []analyzer.PrefixAnalysis{
					{Prefix: "logs/", Status: analyzer.StatusStalePrefix},
					{Prefix: "data/", Status: analyzer.StatusOK},
				},
			},
			"bucket-c": {Status: analyzer.StatusOK},
		},
	}

	findings := FlattenScanFindings(data)
	// bucket-a: MISSING_BUCKET, bucket-b/logs/: STALE_PREFIX
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	types := map[string]bool{}
	for _, f := range findings {
		types[f.Type] = true
	}
	if !types["MISSING_BUCKET"] {
		t.Error("expected MISSING_BUCKET finding")
	}
	if !types["STALE_PREFIX"] {
		t.Error("expected STALE_PREFIX finding")
	}
}

func TestFlattenDiscoveryFindings(t *testing.T) {
	data := report.DiscoveryData{
		Buckets: map[string]*analyzer.BucketDiscovery{
			"unused-1": {Status: analyzer.StatusUnusedBucket},
			"risky-1":  {Status: analyzer.StatusRisky},
			"ok-1":     {Status: analyzer.StatusOK},
		},
	}

	findings := FlattenDiscoveryFindings(data)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestDiff_AllStatuses(t *testing.T) {
	baseline := []Finding{
		{Type: "MISSING_BUCKET", Bucket: "old-missing"},
		{Type: "STALE_PREFIX", Bucket: "shared", Prefix: "logs/"},
		{Type: "UNUSED_BUCKET", Bucket: "resolved"},
	}
	current := []Finding{
		{Type: "MISSING_BUCKET", Bucket: "old-missing"},           // unchanged
		{Type: "STALE_PREFIX", Bucket: "shared", Prefix: "logs/"}, // unchanged
		{Type: "MISSING_BUCKET", Bucket: "new-missing"},           // new
	}

	result := Diff(current, baseline)

	if len(result.New) != 1 || result.New[0].Bucket != "new-missing" {
		t.Errorf("expected 1 new finding (new-missing), got %+v", result.New)
	}
	if len(result.Resolved) != 1 || result.Resolved[0].Bucket != "resolved" {
		t.Errorf("expected 1 resolved finding (resolved), got %+v", result.Resolved)
	}
	if len(result.Unchanged) != 2 {
		t.Errorf("expected 2 unchanged findings, got %d", len(result.Unchanged))
	}
}

func TestDiff_EmptyBaseline(t *testing.T) {
	current := []Finding{{Type: "MISSING_BUCKET", Bucket: "a"}}
	result := Diff(current, nil)
	if len(result.New) != 1 {
		t.Errorf("expected 1 new, got %d", len(result.New))
	}
	if len(result.Resolved) != 0 {
		t.Errorf("expected 0 resolved, got %d", len(result.Resolved))
	}
}

func TestDiff_EmptyCurrent(t *testing.T) {
	baseline := []Finding{{Type: "MISSING_BUCKET", Bucket: "a"}}
	result := Diff(nil, baseline)
	if len(result.Resolved) != 1 {
		t.Errorf("expected 1 resolved, got %d", len(result.Resolved))
	}
	if len(result.New) != 0 {
		t.Errorf("expected 0 new, got %d", len(result.New))
	}
}

func TestLoadScanBaseline(t *testing.T) {
	data := report.Data{
		Tool:    "s3spectre",
		Version: "0.1.0",
		Buckets: map[string]*analyzer.BucketAnalysis{
			"test-bucket": {Status: analyzer.StatusMissingBucket},
		},
	}
	raw, _ := json.Marshal(data)
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := LoadScanBaseline(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestLoadScanBaseline_NotFound(t *testing.T) {
	_, err := LoadScanBaseline("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadScanBaseline_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadScanBaseline(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
