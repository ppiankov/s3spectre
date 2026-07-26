package analyzer

import (
	"fmt"

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

	// Service-managed buckets (CloudTrail, AWS Config, ELB logs, etc.) are
	// excluded from the age/inactivity/empty UNUSED signal only: those
	// heuristics assume neglect, but a managed bucket can legitimately be
	// young, quiet, or transiently empty. Version sprawl, encryption, and
	// public-access checks below still run for managed buckets, since those
	// are independent, legitimate findings unrelated to "is this unused".
	managed := IsServiceManagedBucket(info.Name)

	// Factor 1: Age (20 points if older than threshold)
	if !managed && info.AgeInDays > config.AgeThresholdDays && config.AgeThresholdDays > 0 {
		discovery.RiskScore += 20
		discovery.RiskFactors = append(discovery.RiskFactors,
			fmt.Sprintf("Old bucket (%d days)", info.AgeInDays))
	}

	// Factor 2: Inactivity (50 points base, scaling up for severe staleness so a
	// multi-year-inactive bucket can surface at the default risk threshold on
	// this signal alone, instead of requiring an unrelated second factor)
	if !managed && info.DaysSinceActivity > config.InactivityThresholdDays && config.InactivityThresholdDays > 0 {
		points := 50
		if info.DaysSinceActivity > config.InactivityThresholdDays*5 {
			points = 100
		} else if info.DaysSinceActivity > config.InactivityThresholdDays*2 {
			points = 75
		}
		discovery.RiskScore += points
		discovery.RiskFactors = append(discovery.RiskFactors,
			fmt.Sprintf("No activity for %d days", info.DaysSinceActivity))
		discovery.Recommendations = append(discovery.Recommendations,
			"Consider archiving or deleting if not needed")
	}

	// Factor 3: Empty bucket (30 points)
	if !managed && info.IsEmpty {
		discovery.RiskScore += 30
		discovery.RiskFactors = append(discovery.RiskFactors, "Empty bucket")
		discovery.Recommendations = append(discovery.Recommendations,
			"Delete if not needed")
	}

	// Factor 4: Deprecated tags (20 points)
	if isDeprecated, _, _ := IsDeprecatedTag(info.Tags); isDeprecated {
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
		// Determine specific status. Managed buckets never land on the
		// "delete it" categories (Unused/Inactive) driven by the
		// age/inactivity/empty signal, since that signal was never scored for
		// them; they can still land on VersionSprawl or the generic Risky
		// bucket for real independent findings (e.g. public access).
		switch {
		case !managed && info.IsEmpty && (info.DaysSinceActivity > config.InactivityThresholdDays || info.DaysSinceActivity == 0):
			discovery.Status = StatusUnusedBucket
		case info.VersioningEnabled && info.LifecycleRules == 0:
			discovery.Status = StatusVersionSprawl
		case !managed && info.DaysSinceActivity > config.InactivityThresholdDays:
			discovery.Status = StatusInactive
		default:
			discovery.Status = StatusRisky
		}
	} else {
		discovery.Status = StatusOK
	}

	if managed {
		discovery.Recommendations = append([]string{managedBucketMessage}, discovery.Recommendations...)
	}

	return discovery
}
