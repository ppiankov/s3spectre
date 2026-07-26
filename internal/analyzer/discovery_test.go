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
		t.Errorf("expected risk score 50 for inactivity just past threshold, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_InactivityFactor_ScalesForModerateStaleness(t *testing.T) {
	// Between 2x and 5x the threshold: 75 points, still below the default
	// 100-point threshold on this factor alone.
	info := &s3.BucketInfo{Name: "stale", DaysSinceActivity: 400}
	config := DiscoveryConfig{InactivityThresholdDays: 180, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 75 {
		t.Errorf("expected risk score 75 for moderate staleness (>2x threshold), got %d", d.RiskScore)
	}
}

// TestAnalyzeBucketDiscovery_InactivityFactor_SurfacesSevereStaleness guards
// against multi-year-inactive buckets (real accounts have shown 1000+ days)
// being silently classified OK because the flat 50-point inactivity signal
// never alone crosses the default 100-point threshold.
func TestAnalyzeBucketDiscovery_InactivityFactor_SurfacesSevereStaleness(t *testing.T) {
	info := &s3.BucketInfo{Name: "ancient", DaysSinceActivity: 1171}
	config := DiscoveryConfig{InactivityThresholdDays: 180, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 100 {
		t.Errorf("expected risk score 100 for severe staleness (>5x threshold), got %d", d.RiskScore)
	}
	if d.Status == StatusOK {
		t.Errorf("expected a bucket inactive for 1171 days to not be classified OK at the default threshold")
	}
}

func TestAnalyzeBucketDiscovery_RiskThresholdConfigurable(t *testing.T) {
	info := &s3.BucketInfo{Name: "stale", DaysSinceActivity: 200}
	config := DiscoveryConfig{InactivityThresholdDays: 180, RiskScoreThreshold: 40}

	d := analyzeBucketDiscovery(info, config)

	if d.Status == StatusOK {
		t.Errorf("expected risk score 50 to cross a lowered threshold of 40, got status %s", d.Status)
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
	// Name deliberately avoids any public-bucket-allowlist pattern (e.g.
	// "public", "webview", "-cdn", "-landing") so this test exercises the
	// raw, non-allowlisted scoring path.
	info := &s3.BucketInfo{
		Name:         "customer-records",
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

// TestAnalyzeDiscovery_VersionedBucketInventory_WithLifecycle mirrors the
// scan-path guard for the discover path: a versioned bucket with lifecycle
// rules configured was previously invisible in discover output too.
func TestAnalyzeDiscovery_VersionedBucketInventory_WithLifecycle(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"well-managed": {Name: "well-managed", VersioningEnabled: true, LifecycleRules: 1},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{RiskScoreThreshold: 100})

	if len(result.Summary.VersionedBuckets) != 1 || result.Summary.VersionedBuckets[0] != "well-managed" {
		t.Errorf("expected well-managed bucket in VersionedBuckets inventory, got %v", result.Summary.VersionedBuckets)
	}
	if len(result.Summary.VersionSprawl) != 0 {
		t.Errorf("expected no VersionSprawl finding for a bucket with lifecycle rules, got %v", result.Summary.VersionSprawl)
	}
}

func TestAnalyzeDiscovery_VersionedBucketInventory_WithoutLifecycle(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"sprawling": {Name: "sprawling", VersioningEnabled: true, LifecycleRules: 0},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{RiskScoreThreshold: 30})

	if len(result.Summary.VersionedBuckets) != 1 || result.Summary.VersionedBuckets[0] != "sprawling" {
		t.Errorf("expected sprawling bucket in VersionedBuckets inventory, got %v", result.Summary.VersionedBuckets)
	}
	if len(result.Summary.VersionSprawl) != 1 || result.Summary.VersionSprawl[0] != "sprawling" {
		t.Errorf("expected sprawling bucket still in VersionSprawl, got %v", result.Summary.VersionSprawl)
	}
}

func TestAnalyzeDiscovery_VersionedBucketInventory_NoVersioning(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"plain": {Name: "plain", VersioningEnabled: false},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{RiskScoreThreshold: 100})

	if len(result.Summary.VersionedBuckets) != 0 {
		t.Errorf("expected no versioned-bucket entry for a non-versioned bucket, got %v", result.Summary.VersionedBuckets)
	}
}

func TestAnalyzeBucketDiscovery_EstimateCost_ComputesOverheadWhenEnabled(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "sprawling",
		Region:            "us-east-1",
		VersioningEnabled: true,
		LifecycleRules:    0,
		TotalSize:         1 * 1024 * 1024 * 1024,  // 1 GiB current
		TotalVersionSize:  11 * 1024 * 1024 * 1024, // 11 GiB across all versions
	}
	config := DiscoveryConfig{EstimateCost: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.EstimatedMonthlyCostUSD <= 0 {
		t.Fatalf("expected a positive cost estimate for 10 GiB of overhead, got %v", d.EstimatedMonthlyCostUSD)
	}
}

// TestAnalyzeBucketDiscovery_EstimateCost_OffByDefault guards against the
// opt-in flag changing behavior when not explicitly enabled.
func TestAnalyzeBucketDiscovery_EstimateCost_OffByDefault(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "sprawling",
		Region:            "us-east-1",
		VersioningEnabled: true,
		LifecycleRules:    0,
		TotalSize:         1 * 1024 * 1024 * 1024,
		TotalVersionSize:  11 * 1024 * 1024 * 1024,
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100} // EstimateCost left false

	d := analyzeBucketDiscovery(info, config)

	if d.EstimatedMonthlyCostUSD != 0 {
		t.Fatalf("expected no cost estimate when EstimateCost is off, got %v", d.EstimatedMonthlyCostUSD)
	}
}

// TestAnalyzeBucketDiscovery_PublicAccess_AllowlistedNameReducedScore guards
// against a bucket whose name suggests intentionally-public content (e.g.
// *-web-public) contributing full severity, drowning out genuine
// misconfigurations found elsewhere in the account.
func TestAnalyzeBucketDiscovery_PublicAccess_AllowlistedNameReducedScore(t *testing.T) {
	info := &s3.BucketInfo{
		Name:         "myapp-web-public",
		PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
	}
	config := DiscoveryConfig{CheckPublicAccess: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 30 {
		t.Fatalf("expected reduced risk score 30 for an allowlisted public bucket name, got %d", d.RiskScore)
	}
}

// TestAnalyzeBucketDiscovery_PublicAccess_NonAllowlistedNameFullScore
// confirms the allowlist reduction is scoped to matching names only; a
// bucket with no naming signal keeps the full 60-point severity.
func TestAnalyzeBucketDiscovery_PublicAccess_NonAllowlistedNameFullScore(t *testing.T) {
	info := &s3.BucketInfo{
		Name:         "internal-recommendation-service",
		PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
	}
	config := DiscoveryConfig{CheckPublicAccess: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 60 {
		t.Fatalf("expected full risk score 60 for a non-allowlisted public bucket, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_PublicAccess_ConfigAllowlistPattern(t *testing.T) {
	info := &s3.BucketInfo{
		Name:         "acme-static-assets",
		PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
	}
	config := DiscoveryConfig{
		CheckPublicAccess:             true,
		RiskScoreThreshold:            100,
		PublicBucketAllowlistPatterns: []string{"static-assets"},
	}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 30 {
		t.Fatalf("expected reduced risk score 30 for a config-supplied allowlist pattern match, got %d", d.RiskScore)
	}
}

// TestAnalyzeDiscovery_PublicBucketInventory_IncludesAllowlistedBuckets
// guards against the naming allowlist silently dropping a bucket from
// evidence -- reduced severity must not mean invisible.
func TestAnalyzeDiscovery_PublicBucketInventory_IncludesAllowlistedBuckets(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"allowlisted-web-public": {
			Name:         "allowlisted-web-public",
			PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
		},
		"not-public": {
			Name:         "not-public",
			PublicAccess: &s3.PublicAccessInfo{IsPublic: false},
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{CheckPublicAccess: true, RiskScoreThreshold: 100})

	if len(result.Summary.PublicBuckets) != 1 || result.Summary.PublicBuckets[0] != "allowlisted-web-public" {
		t.Fatalf("expected allowlisted public bucket in PublicBuckets inventory regardless of reduced severity, got %v", result.Summary.PublicBuckets)
	}
}

// TestAnalyzeDiscovery_PublicBucketInventory_PopulatedIndependentOfCheckFlag
// mirrors the VersionedBuckets design: the inventory is evidence, populated
// whenever the underlying data says public, not gated by whether the
// operator asked for scoring via --check-public.
func TestAnalyzeDiscovery_PublicBucketInventory_PopulatedIndependentOfCheckFlag(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"public-bucket": {
			Name:         "public-bucket",
			PublicAccess: &s3.PublicAccessInfo{IsPublic: true},
		},
	}

	result := AnalyzeDiscovery(buckets, DiscoveryConfig{CheckPublicAccess: false, RiskScoreThreshold: 100})

	if len(result.Summary.PublicBuckets) != 1 {
		t.Fatalf("expected public bucket in inventory even when CheckPublicAccess scoring is disabled, got %v", result.Summary.PublicBuckets)
	}
	if result.Buckets["public-bucket"].RiskScore != 0 {
		t.Fatalf("expected no scoring contribution when CheckPublicAccess is disabled, got score %d", result.Buckets["public-bucket"].RiskScore)
	}
}

// TestAnalyzeBucketDiscovery_EstimateCost_InactiveBucketStorage guards the
// WO-43 extension: an inactive bucket's own full storage size should be
// priced, not just version-sprawl overhead.
func TestAnalyzeBucketDiscovery_EstimateCost_InactiveBucketStorage(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "stale-archive",
		Region:            "us-east-1",
		DaysSinceActivity: 1000,
		TotalSize:         5 * 1024 * 1024 * 1024,
	}
	config := DiscoveryConfig{
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      50,
		EstimateCost:            true,
	}

	d := analyzeBucketDiscovery(info, config)

	if d.Status != StatusInactive {
		t.Fatalf("expected status %s, got %s", StatusInactive, d.Status)
	}
	if d.EstimatedStorageCostUSD <= 0 {
		t.Fatalf("expected a positive storage cost estimate for an inactive bucket, got %v", d.EstimatedStorageCostUSD)
	}
	if d.EstimatedMonthlyCostUSD != 0 {
		t.Fatalf("expected version-sprawl cost field to stay zero for a non-version-sprawl bucket, got %v", d.EstimatedMonthlyCostUSD)
	}
}

func TestAnalyzeBucketDiscovery_EstimateCost_UnusedBucketStorage(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "empty-and-old",
		Region:            "us-east-1",
		IsEmpty:           true,
		DaysSinceActivity: 200,
		TotalSize:         2 * 1024 * 1024 * 1024,
	}
	config := DiscoveryConfig{
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      30,
		EstimateCost:            true,
	}

	d := analyzeBucketDiscovery(info, config)

	if d.Status != StatusUnusedBucket {
		t.Fatalf("expected status %s, got %s", StatusUnusedBucket, d.Status)
	}
	if d.EstimatedStorageCostUSD <= 0 {
		t.Fatalf("expected a positive storage cost estimate for an unused bucket, got %v", d.EstimatedStorageCostUSD)
	}
}

// TestAnalyzeBucketDiscovery_EstimateCost_NoDoubleCountingWithVersionSprawl
// guards against a bucket that is BOTH stale AND version-sprawling getting
// costed under both fields -- a bucket only ever has one Status, so only
// one cost field should ever populate for it.
func TestAnalyzeBucketDiscovery_EstimateCost_NoDoubleCountingWithVersionSprawl(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "sprawling-and-stale",
		Region:            "us-east-1",
		DaysSinceActivity: 1000,
		VersioningEnabled: true,
		LifecycleRules:    0,
		TotalSize:         1 * 1024 * 1024 * 1024,
		TotalVersionSize:  11 * 1024 * 1024 * 1024,
	}
	config := DiscoveryConfig{
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      50,
		EstimateCost:            true,
	}

	d := analyzeBucketDiscovery(info, config)

	if d.Status != StatusVersionSprawl {
		t.Fatalf("expected VersionSprawl to take precedence in status classification, got %s", d.Status)
	}
	if d.EstimatedMonthlyCostUSD <= 0 {
		t.Fatalf("expected version-sprawl overhead cost to be populated, got %v", d.EstimatedMonthlyCostUSD)
	}
	if d.EstimatedStorageCostUSD != 0 {
		t.Fatalf("expected no storage cost field populated for a version-sprawl-classified bucket (avoid double counting), got %v", d.EstimatedStorageCostUSD)
	}
}

// TestAnalyzeBucketDiscovery_EstimateCost_NoDoubleCountingWhenUnusedWinsClassification
// guards a narrower variant of the double-counting bug: the version-sprawl
// raw condition (VersioningEnabled && LifecycleRules == 0) triggers
// EstimatedMonthlyCostUSD independent of the bucket's final Status. When the
// bucket is ALSO empty and stale enough to classify as UnusedBucket (which
// the status switch checks before VersionSprawl), a naive Status-only guard
// on EstimatedStorageCostUSD would populate BOTH cost fields for the same
// bucket. Caught by independent review; regression-tests the fix directly.
func TestAnalyzeBucketDiscovery_EstimateCost_NoDoubleCountingWhenUnusedWinsClassification(t *testing.T) {
	info := &s3.BucketInfo{
		Name:              "empty-sprawl-and-stale",
		Region:            "us-east-1",
		IsEmpty:           true,
		DaysSinceActivity: 1000,
		VersioningEnabled: true,
		LifecycleRules:    0,
		TotalSize:         1 * 1024 * 1024 * 1024,
		TotalVersionSize:  11 * 1024 * 1024 * 1024,
	}
	config := DiscoveryConfig{
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      50,
		EstimateCost:            true,
	}

	d := analyzeBucketDiscovery(info, config)

	if d.Status != StatusUnusedBucket {
		t.Fatalf("expected UnusedBucket to take precedence in status classification, got %s", d.Status)
	}
	if d.EstimatedMonthlyCostUSD <= 0 {
		t.Fatalf("expected version-sprawl overhead cost to still be populated (raw condition is Status-independent), got %v", d.EstimatedMonthlyCostUSD)
	}
	if d.EstimatedStorageCostUSD != 0 {
		t.Fatalf("expected NO storage cost field populated when the overhead cost already covers this bucket, got %v (this was the double-counting bug)", d.EstimatedStorageCostUSD)
	}
}

func TestBucketDiscovery_CostUSD_PrefersVersionSprawlField(t *testing.T) {
	d := &BucketDiscovery{EstimatedMonthlyCostUSD: 5, EstimatedStorageCostUSD: 10}
	if got := d.CostUSD(); got != 5 {
		t.Fatalf("expected CostUSD to prefer EstimatedMonthlyCostUSD, got %v", got)
	}
}

func TestBucketDiscovery_CostUSD_FallsBackToStorageField(t *testing.T) {
	d := &BucketDiscovery{EstimatedStorageCostUSD: 10}
	if got := d.CostUSD(); got != 10 {
		t.Fatalf("expected CostUSD to fall back to EstimatedStorageCostUSD, got %v", got)
	}
}

// TestAnalyzeBucketDiscovery_LifecycleSuggestion_OnlyWhenEnabledAndSprawling
// guards the opt-in flag: no suggestion attached unless both the flag is on
// and the bucket is a genuine version-sprawl finding.
func TestAnalyzeBucketDiscovery_LifecycleSuggestion_OnlyWhenEnabledAndSprawling(t *testing.T) {
	sprawling := &s3.BucketInfo{Name: "sprawling", VersioningEnabled: true, LifecycleRules: 0}

	withFlag := analyzeBucketDiscovery(sprawling, DiscoveryConfig{RiskScoreThreshold: 100, SuggestLifecyclePolicy: true})
	if withFlag.LifecyclePolicySuggestion == nil {
		t.Fatal("expected a lifecycle suggestion for a version-sprawl bucket when the flag is enabled")
	}

	withoutFlag := analyzeBucketDiscovery(sprawling, DiscoveryConfig{RiskScoreThreshold: 100, SuggestLifecyclePolicy: false})
	if withoutFlag.LifecyclePolicySuggestion != nil {
		t.Fatal("expected no lifecycle suggestion when the flag is disabled (opt-in must be off by default)")
	}

	healthy := &s3.BucketInfo{Name: "healthy"}
	healthyResult := analyzeBucketDiscovery(healthy, DiscoveryConfig{RiskScoreThreshold: 100, SuggestLifecyclePolicy: true})
	if healthyResult.LifecyclePolicySuggestion != nil {
		t.Fatal("expected no lifecycle suggestion for a bucket with no version-sprawl finding")
	}
}

// TestAnalyzeDiscovery_TotalEstimatedCostUSD_SumsAcrossBuckets guards the
// WO-45 rollup: the account-level total must equal the sum of every
// bucket's CostUSD(), not just one bucket's figure.
func TestAnalyzeDiscovery_TotalEstimatedCostUSD_SumsAcrossBuckets(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"sprawling": {
			Name:              "sprawling",
			Region:            "us-east-1",
			VersioningEnabled: true,
			LifecycleRules:    0,
			TotalSize:         1 * 1024 * 1024 * 1024,
			TotalVersionSize:  11 * 1024 * 1024 * 1024,
		},
		"stale": {
			Name:              "stale",
			Region:            "us-east-1",
			DaysSinceActivity: 1000,
			TotalSize:         5 * 1024 * 1024 * 1024,
		},
	}
	config := DiscoveryConfig{
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      50,
		EstimateCost:            true,
	}

	result := AnalyzeDiscovery(buckets, config)

	sprawlCost := result.Buckets["sprawling"].CostUSD()
	staleCost := result.Buckets["stale"].CostUSD()
	if sprawlCost <= 0 || staleCost <= 0 {
		t.Fatalf("expected both buckets to have a nonzero cost estimate, got sprawl=%v stale=%v", sprawlCost, staleCost)
	}
	want := sprawlCost + staleCost
	if result.Summary.TotalEstimatedCostUSD != want {
		t.Fatalf("expected TotalEstimatedCostUSD=%v (sum of per-bucket costs), got %v", want, result.Summary.TotalEstimatedCostUSD)
	}
}

// TestAnalyzeDiscovery_TotalEstimatedCostUSD_ZeroWhenDisabled guards against
// the rollup implying pricing coverage the tool doesn't have when
// --estimate-cost was never passed.
func TestAnalyzeDiscovery_TotalEstimatedCostUSD_ZeroWhenDisabled(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"sprawling": {
			Name:              "sprawling",
			VersioningEnabled: true,
			LifecycleRules:    0,
			TotalSize:         1 * 1024 * 1024 * 1024,
			TotalVersionSize:  11 * 1024 * 1024 * 1024,
		},
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100} // EstimateCost left false

	result := AnalyzeDiscovery(buckets, config)

	if result.Summary.TotalEstimatedCostUSD != 0 {
		t.Fatalf("expected TotalEstimatedCostUSD=0 when EstimateCost is off, got %v", result.Summary.TotalEstimatedCostUSD)
	}
}

func TestAnalyzeDiscovery_GroupByTag_Disabled(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"a": {Name: "a", Tags: map[string]string{"Team": "backend"}},
	}
	result := AnalyzeDiscovery(buckets, DiscoveryConfig{RiskScoreThreshold: 100})

	if result.Summary.TagRollup != nil {
		t.Fatalf("expected no TagRollup when GroupByTag is empty, got %v", result.Summary.TagRollup)
	}
}

// TestAnalyzeDiscovery_GroupByTag_AggregatesByValue guards the core WO-46
// behavior: buckets sharing a tag value must aggregate bucket count and
// summed risk score under that one tag value.
func TestAnalyzeDiscovery_GroupByTag_AggregatesByValue(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"a": {Name: "a", Tags: map[string]string{"Team": "backend"}, IsEmpty: true, DaysSinceActivity: 200},
		"b": {Name: "b", Tags: map[string]string{"Team": "backend"}, AgeInDays: 500},
		"c": {Name: "c", Tags: map[string]string{"Team": "frontend"}, AgeInDays: 500},
	}
	config := DiscoveryConfig{
		AgeThresholdDays:        365,
		InactivityThresholdDays: 180,
		RiskScoreThreshold:      100,
		GroupByTag:              "Team",
	}

	result := AnalyzeDiscovery(buckets, config)

	backend, ok := result.Summary.TagRollup["backend"]
	if !ok {
		t.Fatal("expected a 'backend' rollup entry")
	}
	if backend.BucketCount != 2 {
		t.Fatalf("expected 2 buckets under 'backend', got %d", backend.BucketCount)
	}
	wantRisk := result.Buckets["a"].RiskScore + result.Buckets["b"].RiskScore
	if backend.RiskScore != wantRisk {
		t.Fatalf("expected summed risk score %d for 'backend', got %d", wantRisk, backend.RiskScore)
	}

	frontend, ok := result.Summary.TagRollup["frontend"]
	if !ok || frontend.BucketCount != 1 {
		t.Fatalf("expected 1 bucket under 'frontend', got %+v", frontend)
	}
}

// TestAnalyzeDiscovery_GroupByTag_MissingTagGoesToUntagged guards against a
// bucket without the configured tag being silently dropped from the rollup
// instead of landing in an explicit "untagged" bucket.
func TestAnalyzeDiscovery_GroupByTag_MissingTagGoesToUntagged(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"no-team-tag":    {Name: "no-team-tag", Tags: map[string]string{"Environment": "prod"}},
		"no-tags-at-all": {Name: "no-tags-at-all"},
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100, GroupByTag: "Team"}

	result := AnalyzeDiscovery(buckets, config)

	untagged, ok := result.Summary.TagRollup["untagged"]
	if !ok {
		t.Fatal("expected an 'untagged' rollup entry")
	}
	if untagged.BucketCount != 2 {
		t.Fatalf("expected 2 buckets under 'untagged', got %d", untagged.BucketCount)
	}
}

func TestIsDefaultKMSKey(t *testing.T) {
	cases := []struct {
		keyID string
		want  bool
	}{
		{"arn:aws:kms:eu-central-1:123456789012:alias/aws/s3", true},
		{"alias/aws/s3", true},
		{"arn:aws:kms:eu-central-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab", false},
		{"arn:aws:kms:eu-central-1:123456789012:alias/my-custom-key", false},
		{"", false},
	}
	for _, tt := range cases {
		if got := isDefaultKMSKey(tt.keyID); got != tt.want {
			t.Errorf("isDefaultKMSKey(%q) = %v, want %v", tt.keyID, got, tt.want)
		}
	}
}

func TestAnalyzeBucketDiscovery_DefaultKMSKey_ReducedFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name: "kms-default",
		Encryption: &s3.EncryptionInfo{
			Enabled:        true,
			Algorithm:      "aws:kms",
			KMSMasterKeyID: "arn:aws:kms:eu-central-1:123456789012:alias/aws/s3",
		},
	}
	config := DiscoveryConfig{CheckEncryption: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 15 {
		t.Fatalf("expected risk score 15 for default-KMS-key bucket, got %d", d.RiskScore)
	}
}

func TestAnalyzeBucketDiscovery_CustomerManagedKMSKey_NoFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name: "kms-cmk",
		Encryption: &s3.EncryptionInfo{
			Enabled:        true,
			Algorithm:      "aws:kms",
			KMSMasterKeyID: "arn:aws:kms:eu-central-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab",
		},
	}
	config := DiscoveryConfig{CheckEncryption: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 0 {
		t.Fatalf("expected risk score 0 for customer-managed-key bucket, got %d", d.RiskScore)
	}
}

// TestAnalyzeBucketDiscovery_SSES3_NoDefaultKeyFactor guards against SSE-S3
// (AES256, no KMS at all) being mistakenly flagged by the CMK-vs-default
// check, which only applies to aws:kms.
func TestAnalyzeBucketDiscovery_SSES3_NoDefaultKeyFactor(t *testing.T) {
	info := &s3.BucketInfo{
		Name: "sse-s3",
		Encryption: &s3.EncryptionInfo{
			Enabled:   true,
			Algorithm: "AES256",
		},
	}
	config := DiscoveryConfig{CheckEncryption: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 0 {
		t.Fatalf("expected risk score 0 for SSE-S3 bucket (no CMK expectation), got %d", d.RiskScore)
	}
}

// TestAnalyzeBucketDiscovery_NoEncryption_UnaffectedByKMSCheck guards
// against the new default-key factor firing (or double-firing) alongside
// the existing no-encryption factor.
func TestAnalyzeBucketDiscovery_NoEncryption_UnaffectedByKMSCheck(t *testing.T) {
	info := &s3.BucketInfo{
		Name:       "unencrypted",
		Encryption: &s3.EncryptionInfo{Enabled: false},
	}
	config := DiscoveryConfig{CheckEncryption: true, RiskScoreThreshold: 100}

	d := analyzeBucketDiscovery(info, config)

	if d.RiskScore != 40 {
		t.Fatalf("expected risk score 40 (no-encryption factor only, no double count with KMS factor), got %d", d.RiskScore)
	}
}

// TestAnalyzeDiscovery_GroupByTag_LiteralUntaggedValueCollidesWithFallback
// pins a known, accepted edge case: a bucket tagged with the literal value
// "untagged" merges into the same rollup entry as buckets genuinely missing
// the tag. This is documented in cli-reference.md as an accepted limitation
// rather than disambiguated -- this test exists so a future change to the
// merge behavior is a deliberate decision, not a silent regression.
func TestAnalyzeDiscovery_GroupByTag_LiteralUntaggedValueCollidesWithFallback(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"literally-tagged-untagged": {Name: "literally-tagged-untagged", Tags: map[string]string{"Team": "untagged"}},
		"missing-tag-entirely":      {Name: "missing-tag-entirely"},
	}
	config := DiscoveryConfig{RiskScoreThreshold: 100, GroupByTag: "Team"}

	result := AnalyzeDiscovery(buckets, config)

	if len(result.Summary.TagRollup) != 1 {
		t.Fatalf("expected the literal 'untagged' tag value to collide with the missing-tag fallback into one entry, got %d entries: %v", len(result.Summary.TagRollup), result.Summary.TagRollup)
	}
	if g := result.Summary.TagRollup["untagged"]; g == nil || g.BucketCount != 2 {
		t.Fatalf("expected both buckets merged under 'untagged', got %+v", result.Summary.TagRollup["untagged"])
	}
}
