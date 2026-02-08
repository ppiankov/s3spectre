package analyzer

import (
	"testing"

	"github.com/ppiankov/s3spectre/internal/s3"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

func TestAnalyze_MissingBucket(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {Name: "my-bucket", Exists: false},
	}

	result := Analyze(refs, bucketInfo, Config{})

	if result.Summary.TotalBuckets != 1 {
		t.Fatalf("expected 1 total bucket, got %d", result.Summary.TotalBuckets)
	}
	if len(result.Summary.MissingBuckets) != 1 {
		t.Fatalf("expected 1 missing bucket, got %d", len(result.Summary.MissingBuckets))
	}
	if result.Buckets["my-bucket"].Status != StatusMissingBucket {
		t.Errorf("expected status %s, got %s", StatusMissingBucket, result.Buckets["my-bucket"].Status)
	}
}

func TestAnalyze_OKBucket(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {Name: "my-bucket", Exists: true},
	}

	result := Analyze(refs, bucketInfo, Config{})

	if result.Summary.OKBuckets != 1 {
		t.Fatalf("expected 1 OK bucket, got %d", result.Summary.OKBuckets)
	}
	if result.Buckets["my-bucket"].Status != StatusOK {
		t.Errorf("expected status %s, got %s", StatusOK, result.Buckets["my-bucket"].Status)
	}
}

func TestAnalyze_VersionSprawl(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {
			Name:              "my-bucket",
			Exists:            true,
			VersioningEnabled: true,
			LifecycleRules:    0,
		},
	}

	result := Analyze(refs, bucketInfo, Config{})

	if len(result.Summary.VersionSprawl) != 1 {
		t.Fatalf("expected 1 version sprawl bucket, got %d", len(result.Summary.VersionSprawl))
	}
	if result.Buckets["my-bucket"].Status != StatusVersionSprawl {
		t.Errorf("expected status %s, got %s", StatusVersionSprawl, result.Buckets["my-bucket"].Status)
	}
}

func TestAnalyze_VersioningWithLifecycleIsOK(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {
			Name:              "my-bucket",
			Exists:            true,
			VersioningEnabled: true,
			LifecycleRules:    2,
		},
	}

	result := Analyze(refs, bucketInfo, Config{})

	if result.Buckets["my-bucket"].Status != StatusOK {
		t.Errorf("expected status %s, got %s", StatusOK, result.Buckets["my-bucket"].Status)
	}
}

func TestAnalyze_LifecycleMisconfig(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {
			Name:   "my-bucket",
			Exists: true,
			Prefixes: []s3.PrefixInfo{
				{Prefix: "data/", Exists: true, ObjectCount: 500, DaysSinceModified: 10},
			},
		},
	}

	result := Analyze(refs, bucketInfo, Config{StaleThresholdDays: 90})

	if result.Buckets["my-bucket"].Status != StatusLifecycleMisconfig {
		t.Errorf("expected status %s, got %s", StatusLifecycleMisconfig, result.Buckets["my-bucket"].Status)
	}
}

func TestAnalyze_SmallPrefixNoLifecycleIsOK(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {
			Name:   "my-bucket",
			Exists: true,
			Prefixes: []s3.PrefixInfo{
				{Prefix: "data/", Exists: true, ObjectCount: 5, DaysSinceModified: 10},
			},
		},
	}

	result := Analyze(refs, bucketInfo, Config{StaleThresholdDays: 90})

	if result.Buckets["my-bucket"].Status != StatusOK {
		t.Errorf("expected status %s, got %s", StatusOK, result.Buckets["my-bucket"].Status)
	}
}

func TestAnalyze_UnusedBucket(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "other-bucket", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"unused-bucket": {
			Name:    "unused-bucket",
			Exists:  true,
			IsEmpty: true,
		},
	}

	result := Analyze(refs, bucketInfo, Config{
		CheckUnused:          true,
		UnusedScoreThreshold: 150,
	})

	if len(result.Summary.UnusedBuckets) != 1 {
		t.Fatalf("expected 1 unused bucket, got %d", len(result.Summary.UnusedBuckets))
	}
	if result.Buckets["unused-bucket"].Status != StatusUnusedBucket {
		t.Errorf("expected status %s, got %s", StatusUnusedBucket, result.Buckets["unused-bucket"].Status)
	}
}

func TestAnalyze_UnusedCheckDisabled(t *testing.T) {
	bucketInfo := map[string]*s3.BucketInfo{
		"unused-bucket": {
			Name:    "unused-bucket",
			Exists:  true,
			IsEmpty: true,
		},
	}

	result := Analyze(nil, bucketInfo, Config{CheckUnused: false})

	if result.Buckets["unused-bucket"].Status != StatusOK {
		t.Errorf("expected status %s when unused check disabled, got %s", StatusOK, result.Buckets["unused-bucket"].Status)
	}
}

func TestAnalyze_PrefixStatuses(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "my-bucket", Prefix: "logs/", File: "app.py", Line: 10},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"my-bucket": {
			Name:   "my-bucket",
			Exists: true,
			Prefixes: []s3.PrefixInfo{
				{Prefix: "missing/", Exists: false},
				{Prefix: "stale/", Exists: true, ObjectCount: 10, DaysSinceModified: 200},
				{Prefix: "fresh/", Exists: true, ObjectCount: 10, DaysSinceModified: 5},
			},
		},
	}

	result := Analyze(refs, bucketInfo, Config{StaleThresholdDays: 90})

	bucket := result.Buckets["my-bucket"]
	if len(bucket.Prefixes) != 3 {
		t.Fatalf("expected 3 prefixes, got %d", len(bucket.Prefixes))
	}

	prefixByName := make(map[string]PrefixAnalysis)
	for _, p := range bucket.Prefixes {
		prefixByName[p.Prefix] = p
	}

	if prefixByName["missing/"].Status != StatusMissingPrefix {
		t.Errorf("expected missing/ status %s, got %s", StatusMissingPrefix, prefixByName["missing/"].Status)
	}
	if prefixByName["stale/"].Status != StatusStalePrefix {
		t.Errorf("expected stale/ status %s, got %s", StatusStalePrefix, prefixByName["stale/"].Status)
	}
	if prefixByName["fresh/"].Status != StatusOK {
		t.Errorf("expected fresh/ status %s, got %s", StatusOK, prefixByName["fresh/"].Status)
	}

	if len(result.Summary.MissingPrefixes) != 1 {
		t.Errorf("expected 1 missing prefix in summary, got %d", len(result.Summary.MissingPrefixes))
	}
	if len(result.Summary.StalePrefixes) != 1 {
		t.Errorf("expected 1 stale prefix in summary, got %d", len(result.Summary.StalePrefixes))
	}
}

func TestAnalyze_MultipleBuckets(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "existing", File: "app.py", Line: 1},
		{Bucket: "missing", File: "app.py", Line: 2},
	}
	bucketInfo := map[string]*s3.BucketInfo{
		"existing": {Name: "existing", Exists: true},
		"missing":  {Name: "missing", Exists: false},
	}

	result := Analyze(refs, bucketInfo, Config{})

	if result.Summary.TotalBuckets != 2 {
		t.Fatalf("expected 2 total buckets, got %d", result.Summary.TotalBuckets)
	}
	if result.Summary.OKBuckets != 1 {
		t.Errorf("expected 1 OK bucket, got %d", result.Summary.OKBuckets)
	}
	if len(result.Summary.MissingBuckets) != 1 {
		t.Errorf("expected 1 missing bucket, got %d", len(result.Summary.MissingBuckets))
	}
}

func TestCalculateUnusedScore_NotInCode(t *testing.T) {
	info := &s3.BucketInfo{Name: "orphan", Exists: true}
	refs := map[string]bool{}

	score := calculateUnusedScore("orphan", info, refs, Config{UnusedScoreThreshold: 150})

	if score.NotInCode != 100 {
		t.Errorf("expected NotInCode=100, got %d", score.NotInCode)
	}
	if score.Total != 100 {
		t.Errorf("expected Total=100, got %d", score.Total)
	}
	if score.IsUnused {
		t.Error("expected IsUnused=false with score 100 < threshold 150")
	}
}

func TestCalculateUnusedScore_NotInCodeAndEmpty(t *testing.T) {
	info := &s3.BucketInfo{Name: "orphan", Exists: true, IsEmpty: true}
	refs := map[string]bool{}

	score := calculateUnusedScore("orphan", info, refs, Config{UnusedScoreThreshold: 150})

	if score.Total != 150 {
		t.Errorf("expected Total=150, got %d", score.Total)
	}
	if !score.IsUnused {
		t.Error("expected IsUnused=true with score 150 >= threshold 150")
	}
}

func TestCalculateUnusedScore_DeprecatedTag(t *testing.T) {
	info := &s3.BucketInfo{
		Name:   "tagged",
		Exists: true,
		Tags:   map[string]string{"status": "Deprecated"},
	}
	refs := map[string]bool{"tagged": true}

	score := calculateUnusedScore("tagged", info, refs, Config{UnusedScoreThreshold: 150})

	if score.DeprecatedTag != 20 {
		t.Errorf("expected DeprecatedTag=20, got %d", score.DeprecatedTag)
	}
	if score.Total != 20 {
		t.Errorf("expected Total=20, got %d", score.Total)
	}
}

func TestCalculateUnusedScore_ReferencedBucketNoFactors(t *testing.T) {
	info := &s3.BucketInfo{Name: "active", Exists: true}
	refs := map[string]bool{"active": true}

	score := calculateUnusedScore("active", info, refs, Config{UnusedScoreThreshold: 150})

	if score.Total != 0 {
		t.Errorf("expected Total=0 for active bucket, got %d", score.Total)
	}
	if score.IsUnused {
		t.Error("expected IsUnused=false for active bucket")
	}
}

func TestCalculateUnusedScore_DefaultThreshold(t *testing.T) {
	info := &s3.BucketInfo{Name: "orphan", Exists: true, IsEmpty: true}
	refs := map[string]bool{}

	score := calculateUnusedScore("orphan", info, refs, Config{UnusedScoreThreshold: 0})

	// Default threshold is 150, score is 150 (100 + 50)
	if !score.IsUnused {
		t.Error("expected IsUnused=true with default threshold 150")
	}
}

func TestFilterRefsByBucket(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "a", File: "f1", Line: 1},
		{Bucket: "b", File: "f2", Line: 2},
		{Bucket: "a", File: "f3", Line: 3},
	}

	filtered := filterRefsByBucket(refs, "a")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 refs for bucket 'a', got %d", len(filtered))
	}
}

func TestFilterRefsByBucket_NoMatch(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "a", File: "f1", Line: 1},
	}

	filtered := filterRefsByBucket(refs, "nonexistent")

	if len(filtered) != 0 {
		t.Fatalf("expected 0 refs, got %d", len(filtered))
	}
}
