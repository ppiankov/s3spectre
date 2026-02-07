package analyzer

import (
	"fmt"
	"strings"

	"github.com/ppiankov/s3spectre/internal/s3"
)

// DiscoveryConfig contains configuration for discovery analysis
type DiscoveryConfig struct {
	AgeThresholdDays        int
	InactivityThresholdDays int
	CheckEncryption         bool
	CheckPublicAccess       bool
	RiskScoreThreshold      int
}

// DiscoveryResult contains discovery analysis results
type DiscoveryResult struct {
	Buckets map[string]*BucketDiscovery `json:"buckets"`
	Summary DiscoverySummary            `json:"summary"`
}

// BucketDiscovery contains discovery analysis for a bucket
type BucketDiscovery struct {
	Name            string         `json:"name"`
	Region          string         `json:"region"`
	Status          Status         `json:"status"`
	RiskScore       int            `json:"risk_score"`
	RiskFactors     []string       `json:"risk_factors"`
	Recommendations []string       `json:"recommendations"`
	BucketInfo      *s3.BucketInfo `json:"bucket_info,omitempty"`
}

// DiscoverySummary contains high-level summary
type DiscoverySummary struct {
	TotalBuckets    int      `json:"total_buckets"`
	HealthyBuckets  int      `json:"healthy_buckets"`
	UnusedBuckets   []string `json:"unused_buckets,omitempty"`
	RiskyBuckets    []string `json:"risky_buckets,omitempty"`
	InactiveBuckets []string `json:"inactive_buckets,omitempty"`
	VersionSprawl   []string `json:"version_sprawl,omitempty"`
	TotalRegions    int      `json:"total_regions"`
}

// AnalyzeDiscovery analyzes buckets discovered from AWS
func AnalyzeDiscovery(buckets map[string]*s3.BucketInfo, config DiscoveryConfig) *DiscoveryResult {
	result := &DiscoveryResult{
		Buckets: make(map[string]*BucketDiscovery),
		Summary: DiscoverySummary{},
	}

	regions := make(map[string]bool)

	for name, info := range buckets {
		discovery := analyzeBucketDiscovery(info, config)
		result.Buckets[name] = discovery

		// Track regions
		if info.Region != "" {
			regions[info.Region] = true
		}

		// Update summary
		result.Summary.TotalBuckets++

		switch discovery.Status {
		case StatusOK:
			result.Summary.HealthyBuckets++
		case StatusUnusedBucket:
			result.Summary.UnusedBuckets = append(result.Summary.UnusedBuckets, name)
		case StatusRisky:
			result.Summary.RiskyBuckets = append(result.Summary.RiskyBuckets, name)
		case StatusInactive:
			result.Summary.InactiveBuckets = append(result.Summary.InactiveBuckets, name)
		case StatusVersionSprawl:
			result.Summary.VersionSprawl = append(result.Summary.VersionSprawl, name)
		}
	}

	result.Summary.TotalRegions = len(regions)
	return result
}

// analyzeBucketDiscovery analyzes a single bucket
func analyzeBucketDiscovery(info *s3.BucketInfo, config DiscoveryConfig) *BucketDiscovery {
	discovery := &BucketDiscovery{
		Name:            info.Name,
		Region:          info.Region,
		RiskScore:       0,
		RiskFactors:     make([]string, 0),
		Recommendations: make([]string, 0),
		BucketInfo:      info,
	}

	// Factor 1: Age (20 points if older than threshold)
	if info.AgeInDays > config.AgeThresholdDays && config.AgeThresholdDays > 0 {
		discovery.RiskScore += 20
		discovery.RiskFactors = append(discovery.RiskFactors,
			fmt.Sprintf("Old bucket (%d days)", info.AgeInDays))
	}

	// Factor 2: Inactivity (50 points if no activity)
	if info.DaysSinceActivity > config.InactivityThresholdDays && config.InactivityThresholdDays > 0 {
		discovery.RiskScore += 50
		discovery.RiskFactors = append(discovery.RiskFactors,
			fmt.Sprintf("No activity for %d days", info.DaysSinceActivity))
		discovery.Recommendations = append(discovery.Recommendations,
			"Consider archiving or deleting if not needed")
	}

	// Factor 3: Empty bucket (30 points)
	if info.IsEmpty {
		discovery.RiskScore += 30
		discovery.RiskFactors = append(discovery.RiskFactors, "Empty bucket")
		discovery.Recommendations = append(discovery.Recommendations,
			"Delete if not needed")
	}

	// Factor 4: Deprecated tags (20 points)
	if hasDeprecatedTags(info.Tags) {
		discovery.RiskScore += 20
		discovery.RiskFactors = append(discovery.RiskFactors, "Has deprecated tags")
		discovery.Recommendations = append(discovery.Recommendations,
			"Verify if bucket is still needed")
	}

	// Factor 5: Version sprawl (30 points)
	if info.VersioningEnabled && info.LifecycleRules == 0 {
		discovery.RiskScore += 30
		discovery.RiskFactors = append(discovery.RiskFactors,
			"Versioning enabled without lifecycle rules")
		discovery.Recommendations = append(discovery.Recommendations,
			"Add lifecycle policy to expire old versions")
	}

	// Factor 6: No encryption (40 points) - if check enabled
	if config.CheckEncryption && info.Encryption != nil && !info.Encryption.Enabled {
		discovery.RiskScore += 40
		discovery.RiskFactors = append(discovery.RiskFactors, "No encryption enabled")
		discovery.Recommendations = append(discovery.Recommendations,
			"Enable default encryption (AES256 or KMS)")
	}

	// Factor 7: Public access (60 points - high risk) - if check enabled
	if config.CheckPublicAccess && info.PublicAccess != nil && info.PublicAccess.IsPublic {
		discovery.RiskScore += 60
		discovery.RiskFactors = append(discovery.RiskFactors, "Public access enabled")
		discovery.Recommendations = append(discovery.Recommendations,
			"Review and restrict public access if not required")
	}

	// Determine status based on risk score and factors
	threshold := config.RiskScoreThreshold
	if threshold <= 0 {
		threshold = 100
	}

	if discovery.RiskScore >= threshold {
		// Determine specific status
		if info.IsEmpty && (info.DaysSinceActivity > config.InactivityThresholdDays || info.DaysSinceActivity == 0) {
			discovery.Status = StatusUnusedBucket
		} else if info.VersioningEnabled && info.LifecycleRules == 0 {
			discovery.Status = StatusVersionSprawl
		} else if info.DaysSinceActivity > config.InactivityThresholdDays {
			discovery.Status = StatusInactive
		} else {
			discovery.Status = StatusRisky
		}
	} else {
		discovery.Status = StatusOK
	}

	return discovery
}

// hasDeprecatedTags checks if bucket has deprecated tags
func hasDeprecatedTags(tags map[string]string) bool {
	if tags == nil {
		return false
	}

	deprecatedTags := []string{"deprecated", "old", "unused", "delete", "obsolete", "legacy", "retired"}

	for key, value := range tags {
		keyLower := strings.ToLower(key)
		valueLower := strings.ToLower(value)

		for _, deprecated := range deprecatedTags {
			if keyLower == deprecated || valueLower == deprecated {
				return true
			}
		}
	}

	return false
}
