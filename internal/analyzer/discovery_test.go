package analyzer

import (
	"testing"

	"github.com/ppiankov/s3spectre/internal/s3"
)

func TestAnalyzeDiscovery_HealthyBucket(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"healthy": {
			Name:   "healthy",
			Exists: true,
			Region: "us-east-1",
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{RiskScoreThreshold: 100})

	if result.Summary.TotalBuckets != 1 {
		t.Fatalf("expected 1 total bucket, got %d", result.Summary.TotalBuckets)
	}
	if result.Summary.HealthyBuckets != 1 {
		t.Errorf("expected 1 healthy bucket, got %d", result.Summary.HealthyBuckets)
	}
	if result.Buckets["healthy"].Status != StatusOK {
		t.Errorf("expected status %s, got %s", StatusOK, result.Buckets["healthy"].Status)
	}
}

func TestAnalyzeDiscovery_RegionTracking(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"a": {Name: "a", Region: "us-east-1"},
		"b": {Name: "b", Region: "eu-west-1"},
		"c": {Name: "c", Region: "us-east-1"},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{RiskScoreThreshold: 100})

	if result.Summary.TotalRegions != 2 {
		t.Errorf("expected 2 regions, got %d", result.Summary.TotalRegions)
	}
}

func TestAnalyzeDiscovery_UnusedBucket(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"empty-old": {
			Name:              "empty-old",
			IsEmpty:           true,
			DaysSinceActivity: 200,
			AgeInDays:         400,
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{
		AgeThresholdDays:        365,
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      100,
	})

	if len(result.Summary.UnusedBuckets) != 1 {
		t.Fatalf("expected 1 unused bucket, got %d", len(result.Summary.UnusedBuckets))
	}
	if result.Buckets["empty-old"].Status != StatusUnusedBucket {
		t.Errorf("expected status %s, got %s", StatusUnusedBucket, result.Buckets["empty-old"].Status)
	}
}

func TestAnalyzeBucketDiscovery_AgeFactor(t *testing.T) {
	info := &s3.BucketInfo{Name: "old", AgeInDays: 500}
	config := DiscoveryConfig{AgeThresholdDays: 365, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 20 {
		t.Errorf("expected risk score 20 for age factor, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_AgeFactorDisabled(t *testing.T) {
	info := &s3.BucketInfo{Name: "old", AgeInDays: 500}
	config := DiscoveryConfig{AgeThresholdDays: 0, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 0 {
		t.Errorf("expected risk score 0 when age threshold is 0, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_InactivityFactor(t *testing.T) {
	info := &s3.BucketInfo{Name: "stale", DaysSinceActivity: 200}
	config := DiscoveryConfig{InactivityThresholdDays: 180, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 50 {
		t.Errorf("expected risk score 50 for inactivity, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_EmptyFactor(t *testing.T) {
	info := &s3.BucketInfo{Name: "empty", IsEmpty: true}
	config := DiscoveryConfig{RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 30 {
		t.Errorf("expected risk score 30 for empty bucket, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_DeprecatedTagsFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name: "tagged",
		Tags: map[string]string{"status": "deprecated"},
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 20 {
		t.Errorf("expected risk score 20 for deprecated tags, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_VersionSprawlFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "versioned",
		VersioningEnabled: true,
		LifecycleRules:    0,
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 30 {
		t.Errorf("expected risk score 30 for version sprawl, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_VersioningWithLifecycleNoFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "versioned",
		VersioningEnabled: true,
		LifecycleRules:    3,
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 0 {
		t.Errorf("expected risk score 0 with lifecycle rules, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_EncryptionFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name:       "unencrypted",
		Encryption: &s3.EncryptionInfo{Enabled: false},
	}
	config := DiscoveryConfig{CheckEncryption: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 40 {
		t.Errorf("expected risk score 40 for no encryption, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_EncryptionCheckDisabled(t *testing.T) {
	info := &s3.BucketInfo{
		Name:       "unencrypted",
		Encryption: &s3.EncryptionInfo{Enabled: false},
	}
	config := DiscoveryConfig{CheckEncryption: false, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 0 {
		t.Errorf("expected risk score 0 when encryption check disabled, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_PublicAccessFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name:         "public",
		PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
	}
	config := DiscoveryConfig{CheckPublicAccess: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 60 {
		t.Errorf("expected risk score 60 for public access, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_PublicAccessCheckDisabled(t *testing.T) {
	info := &s3.BucketInfo{
		Name:         "public",
		PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
	}
	config := DiscoveryConfig{CheckPublicAccess: false, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 0 {
		t.Errorf("expected risk score 0 when public access check disabled, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_CombinedFactors(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "risky",
		IsEmpty:           true,
		AgeInDays:         500,
		DaysSinceActivity: 200,
		VersioningEnabled: true,
		LifecycleRules:    0,
	}
	config := DiscoveryConfig{
		AgeThresholdDays:        365,
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      100,
	}

	d := analyzeBucketDiscovery(info, config)

	// age(20) + inactivity(50) + empty(30) + version_sprawl(30) = 130
	expected := 130
	if d.RiskScore != expected {
		t.Errorf("expected combined risk score %d, got %d", expected, d.RiskScore)
	}
	if d.Status == StatusOK {
		t.Error("expected non-OK status for high risk score")
	}
}

func TestAnalyzeBucketDiscovery_StatusClassification(t *testing.T) {
	tests := []struct {
		name     string
		info     *s3.BucketInfo
		config   DiscoveryConfig
		expected Status
	}{
		{
			name: "unused: empty and inactive",
			info: &s3.BucketInfo{
				Name:              "unused",
				IsEmpty:           true,
				DaysSinceActivity: 200,
			},
			config:   DiscoveryConfig{InactivityThresholdDays: 180, RiskScoreThreshold: 50},
			expected: StatusUnusedBucket,
		},
		{
			name: "version sprawl: versioning without lifecycle, above threshold",
			info: &s3.BucketInfo{
				Name:              "sprawl",
				VersioningEnabled: true,
				LifecycleRules:    0,
				AgeInDays:         500,
				DaysSinceActivity: 10,
			},
			config:   DiscoveryConfig{AgeThresholdDays: 365, RiskScoreThreshold: 50},
			expected: StatusVersionSprawl,
		},
		{
			name: "inactive: only inactivity factor above threshold",
			info: &s3.BucketInfo{
				Name:              "inactive",
				DaysSinceActivity: 400,
				AgeInDays:         500,
			},
			config:   DiscoveryConfig{InactivityThresholdDays: 180, AgeThresholdDays: 365, RiskScoreThreshold: 50},
			expected: StatusInactive,
		},
		{
			name: "risky: public access pushes over threshold",
			info: &s3.BucketInfo{
				Name:         "risky",
				PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
				AgeInDays:    500,
			},
			config:   DiscoveryConfig{CheckPublicAccess: true, AgeThresholdDays: 365, RiskScoreThreshold: 50},
			expected: StatusRisky,
		},
		{
			name: "ok: below threshold",
			info: &s3.BucketInfo{
				Name:      "fine",
				AgeInDays: 100,
			},
			config:   DiscoveryConfig{AgeThresholdDays: 365, RiskScoreThreshold: 100},
			expected: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := analyzeBucketDiscovery(tt.info, tt.config)
			if d.Status != tt.expected {
				t.Errorf("expected status %s, got %s (score=%d, factors=%v)",
					tt.expected, d.Status, d.RiskScore, d.RiskFactors)
			}
		})
	}
}

func TestAnalyzeBucketDiscovery_DefaultThreshold(t *testing.T) {
	info := &s3.BucketInfo{
		Name:      "borderline",
		AgeInDays: 500,
	}
	config := DiscoveryConfig{
		AgeThresholdDays:   365,
		RiskScoreThreshold: 0, // should default to 100
	}

	d := analyzeBucketDiscovery(info, config)

	// Score is only 20 (age), default threshold 100 -> OK
	if d.Status != StatusOK {
		t.Errorf("expected status OK with default threshold, got %s (score=%d)", d.Status, d.RiskScore)
	}
}

func TestHasDeprecatedTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     map[string]string
		expected bool
	}{
		{"nil tags", nil, false},
		{"empty tags", map[string]string{}, false},
		{"no deprecated tags", map[string]string{"env": "prod"}, false},
		{"deprecated key", map[string]string{"deprecated": "true"}, true},
		{"deprecated value", map[string]string{"status": "deprecated"}, true},
		{"case insensitive key", map[string]string{"DEPRECATED": "yes"}, true},
		{"case insensitive value", map[string]string{"status": "OBSOLETE"}, true},
		{"old tag", map[string]string{"old": "yes"}, true},
		{"unused tag", map[string]string{"unused": "true"}, true},
		{"delete tag", map[string]string{"delete": "true"}, true},
		{"legacy tag", map[string]string{"legacy": "true"}, true},
		{"retired tag", map[string]string{"retired": "true"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasDeprecatedTags(tt.tags)
			if got != tt.expected {
				t.Errorf("hasDeprecatedTags(%v) = %v, want %v", tt.tags, got, tt.expected)
			}
		})
	}
}

func TestAnalyzeDiscovery_SummaryCategories(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"healthy": {Name: "healthy", Region: "us-east-1"},
		"empty-inactive": {
			Name:              "empty-inactive",
			Region:            "us-east-1",
			IsEmpty:           true,
			DaysSinceActivity: 200,
		},
		"public-old": {
			Name:         "public-old",
			Region:       "eu-west-1",
			AgeInDays:    500,
			PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{
		AgeThresholdDays:        365,
		InactivityThresholdDays: 180,
		CheckPublicAccess:       true,
		RiskScoreThreshold:      50,
	})

	if result.Summary.TotalBuckets != 3 {
		t.Errorf("expected 3 total, got %d", result.Summary.TotalBuckets)
	}
	if result.Summary.HealthyBuckets != 1 {
		t.Errorf("expected 1 healthy, got %d", result.Summary.HealthyBuckets)
	}
	if result.Summary.TotalRegions != 2 {
		t.Errorf("expected 2 regions, got %d", result.Summary.TotalRegions)
	}
}
