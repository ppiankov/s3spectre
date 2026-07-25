package analyzer

import (
	"testing"

	"github.com/ppiankov/s3spectre/internal/s3"
)

func TestIsServiceManagedBucket(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"aws-cloudtrail-logs-123456789012-abcd1234", true},
		{"aws-config-bucket-123456789012-us-east-1", true},
		{"elasticloadbalancing-logs-123456789012", true},
		{"cf-templates-abcd1234-us-east-1", true},
		{"my-application-data", false},
		{"legacy-bucket", false},
	}

	for _, tt := range cases {
		if got := IsServiceManagedBucket(tt.name); got != tt.want {
			t.Errorf("IsServiceManagedBucket(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestAnalyze_ManagedBucketSuppressed guards against a service-managed bucket
// (e.g. a CloudTrail log destination) being scored unused and recommended for
// deletion by the scan path, which would break the owning AWS service.
func TestAnalyze_ManagedBucketSuppressed(t *testing.T) {
	bucketInfo := map[string]*s3.BucketInfo{
		"aws-cloudtrail-logs-123456789012-abcd1234": {
			Name:    "aws-cloudtrail-logs-123456789012-abcd1234",
			Exists:  true,
			IsEmpty: true,
		},
	}

	result := Analyze(nil, bucketInfo, Config{
		CheckUnused:          true,
		UnusedScoreThreshold: 150,
	})

	analysis := result.Buckets["aws-cloudtrail-logs-123456789012-abcd1234"]
	if analysis.Status != StatusOK {
		t.Errorf("expected managed bucket status %s, got %s", StatusOK, analysis.Status)
	}
	if analysis.UnusedScore != nil {
		t.Errorf("expected no unused score computed for managed bucket, got %+v", analysis.UnusedScore)
	}
	if len(result.Summary.UnusedBuckets) != 0 {
		t.Errorf("expected managed bucket not counted as unused, got %v", result.Summary.UnusedBuckets)
	}
}

// TestAnalyze_OrdinaryUnusedBucketStillFlagged confirms the managed-bucket
// suppression does not silently disable unused detection for normal buckets
// sharing the same generic empty/no-reference signals.
func TestAnalyze_OrdinaryUnusedBucketStillFlagged(t *testing.T) {
	bucketInfo := map[string]*s3.BucketInfo{
		"my-scratch-bucket": {
			Name:    "my-scratch-bucket",
			Exists:  true,
			IsEmpty: true,
		},
	}

	result := Analyze(nil, bucketInfo, Config{
		CheckUnused:          true,
		UnusedScoreThreshold: 150,
	})

	if result.Buckets["my-scratch-bucket"].Status != StatusUnusedBucket {
		t.Errorf("expected ordinary bucket status %s, got %s", StatusUnusedBucket, result.Buckets["my-scratch-bucket"].Status)
	}
}

// TestAnalyzeDiscovery_ManagedBucketSuppressed mirrors the scan-path guard for
// the discover path.
func TestAnalyzeDiscovery_ManagedBucketSuppressed(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"aws-config-bucket-123456789012-us-east-1": {
			Name:              "aws-config-bucket-123456789012-us-east-1",
			IsEmpty:           true,
			DaysSinceActivity: 400,
			AgeInDays:         400,
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{
		AgeThresholdDays:        365,
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      100,
	})

	discovery := result.Buckets["aws-config-bucket-123456789012-us-east-1"]
	if discovery.Status != StatusOK {
		t.Errorf("expected managed bucket status %s, got %s (score=%d)", StatusOK, discovery.Status, discovery.RiskScore)
	}
	if discovery.RiskScore != 0 {
		t.Errorf("expected managed bucket risk score 0, got %d", discovery.RiskScore)
	}
}

// TestAnalyzeDiscovery_OrdinaryRiskyBucketStillFlagged confirms the
// managed-bucket suppression does not silently disable risk scoring for
// normal buckets sharing the same generic age/inactivity/empty signals.
func TestAnalyzeDiscovery_OrdinaryRiskyBucketStillFlagged(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"my-old-scratch-bucket": {
			Name:              "my-old-scratch-bucket",
			IsEmpty:           true,
			DaysSinceActivity: 400,
			AgeInDays:         400,
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{
		AgeThresholdDays:        365,
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      100,
	})

	discovery := result.Buckets["my-old-scratch-bucket"]
	if discovery.Status == StatusOK {
		t.Errorf("expected ordinary old/inactive/empty bucket to be flagged, got status %s (score=%d)", discovery.Status, discovery.RiskScore)
	}
}
