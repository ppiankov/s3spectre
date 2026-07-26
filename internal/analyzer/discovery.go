package analyzer

import (
	"fmt"
	"strings"

	"github.com/ppiankov/s3spectre/internal/pricing"
	"github.com/ppiankov/s3spectre/internal/remediation"
	"github.com/ppiankov/s3spectre/internal/s3"
)

// DiscoveryConfig contains configuration for discovery analysis
type DiscoveryConfig struct {
	AgeThresholdDays        int
	InactivityThresholdDays int
	CheckEncryption         bool
	CheckPublicAccess       bool
	RiskScoreThreshold      int
	EstimateCost            bool
	// PublicBucketAllowlistPatterns extends (does not replace) the built-in
	// naming patterns used to recognize intentionally-public buckets.
	PublicBucketAllowlistPatterns []string
	// SuggestLifecyclePolicy, when true, attaches a deterministic
	// (JSON + Terraform) lifecycle-rule suggestion to VERSION_SPRAWL
	// findings. Informational only -- s3spectre never calls any AWS write
	// API itself.
	SuggestLifecyclePolicy bool
	// GroupByTag, when non-empty, names a bucket tag key (e.g. "Team") to
	// roll the discover summary up by. Buckets missing the tag are grouped
	// under "untagged". Empty disables the rollup.
	GroupByTag string
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
	// EstimatedMonthlyCostUSD is an approximate monthly cost of the version
	// overhead (TotalVersionSize minus TotalSize) for a VersionSprawl finding.
	// Only populated when DiscoveryConfig.EstimateCost is set; 0 otherwise.
	EstimatedMonthlyCostUSD float64 `json:"estimated_monthly_cost_usd,omitempty"`
	// EstimatedStorageCostUSD is an approximate monthly cost of a bucket's
	// full TotalSize, populated only for Inactive/UnusedBucket findings when
	// DiscoveryConfig.EstimateCost is set. Kept distinct from
	// EstimatedMonthlyCostUSD (version-sprawl overhead only) so the two
	// pricing scopes are never conflated under one ambiguous number.
	// analyzeBucketDiscovery only populates this field when
	// EstimatedMonthlyCostUSD is still zero -- a bucket's raw version-sprawl
	// condition (VersioningEnabled && LifecycleRules == 0) is independent of
	// its final Status, so relying on Status alone to keep these mutually
	// exclusive is not sufficient.
	EstimatedStorageCostUSD float64 `json:"estimated_storage_cost_usd,omitempty"`
	// LifecyclePolicySuggestion is a deterministic, human-reviewed lifecycle
	// rule snippet for a VersionSprawl finding. Only populated when
	// DiscoveryConfig.SuggestLifecyclePolicy is set; suggestion only, never
	// applied by s3spectre itself.
	LifecyclePolicySuggestion *remediation.LifecyclePolicySuggestion `json:"lifecycle_policy_suggestion,omitempty"`
}

// CostUSD returns whichever cost estimate is populated for this bucket
// (version-sprawl overhead or full-bucket storage). At most one is ever
// nonzero -- analyzeBucketDiscovery only sets EstimatedStorageCostUSD when
// EstimatedMonthlyCostUSD is still zero.
func (b *BucketDiscovery) CostUSD() float64 {
	if b.EstimatedMonthlyCostUSD > 0 {
		return b.EstimatedMonthlyCostUSD
	}
	return b.EstimatedStorageCostUSD
}

// DiscoverySummary contains high-level summary
type DiscoverySummary struct {
	TotalBuckets    int      `json:"total_buckets"`
	HealthyBuckets  int      `json:"healthy_buckets"`
	UnusedBuckets   []string `json:"unused_buckets,omitempty"`
	RiskyBuckets    []string `json:"risky_buckets,omitempty"`
	InactiveBuckets []string `json:"inactive_buckets,omitempty"`
	VersionSprawl   []string `json:"version_sprawl,omitempty"`
	// VersionedBuckets lists every bucket with versioning enabled, regardless
	// of lifecycle-rule configuration -- an informational inventory, distinct
	// from VersionSprawl (which only lists the misconfigured subset).
	VersionedBuckets []string `json:"versioned_buckets,omitempty"`
	// PublicBuckets lists every bucket with public access enabled, regardless
	// of naming-allowlist status -- an informational inventory so an
	// allowlisted (reduced-severity) bucket is never silently dropped from
	// evidence. Populated whenever bucket_info.public_access.is_public is
	// true, independent of RiskScoreThreshold and CheckPublicAccess (the
	// underlying API call is unconditional; CheckPublicAccess only gates
	// scoring).
	PublicBuckets []string `json:"public_buckets,omitempty"`
	TotalRegions  int      `json:"total_regions"`
	// TotalEstimatedCostUSD sums CostUSD() across every bucket. Only
	// meaningful when DiscoveryConfig.EstimateCost is set; naturally 0
	// otherwise since every bucket's cost fields stay 0 in that case.
	TotalEstimatedCostUSD float64 `json:"total_estimated_cost_usd,omitempty"`
	// TagRollup groups buckets by the DiscoveryConfig.GroupByTag key,
	// summing bucket count and risk score per tag value. Buckets missing
	// the tag land under "untagged". Only populated when GroupByTag is set.
	TagRollup map[string]*TagGroupSummary `json:"tag_rollup,omitempty"`
}

// TagGroupSummary is a per-tag-value rollup of discover findings, used by
// DiscoverySummary.TagRollup to give an ownership view across a large
// account instead of a single flat bucket list.
type TagGroupSummary struct {
	BucketCount        int `json:"bucket_count"`
	RiskScore          int `json:"risk_score"`
	UnusedCount        int `json:"unused_count"`
	RiskyCount         int `json:"risky_count"`
	InactiveCount      int `json:"inactive_count"`
	VersionSprawlCount int `json:"version_sprawl_count"`
	// AverageRiskScore is RiskScore/BucketCount, computed once the full
	// rollup is known (not incrementally, since an average isn't additive).
	// The raw RiskScore sum is biased toward tag values owning more
	// buckets; this field surfaces per-bucket severity so a small,
	// high-severity group isn't buried under a large, low-severity one.
	AverageRiskScore float64 `json:"average_risk_score"`
}

// untaggedGroupKey is the rollup bucket for buckets missing the configured
// GroupByTag key.
const untaggedGroupKey = "untagged"

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

		if info.VersioningEnabled {
			result.Summary.VersionedBuckets = append(result.Summary.VersionedBuckets, name)
		}

		if info.PublicAccess != nil && info.PublicAccess.IsPublic {
			result.Summary.PublicBuckets = append(result.Summary.PublicBuckets, name)
		}

		if config.EstimateCost {
			result.Summary.TotalEstimatedCostUSD += discovery.CostUSD()
		}

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

		if config.GroupByTag != "" {
			addToTagRollup(&result.Summary, config.GroupByTag, info, discovery)
		}
	}

	result.Summary.TotalRegions = len(regions)

	// AverageRiskScore is computed once per group over the finished rollup,
	// not incrementally in addToTagRollup, since an average isn't additive
	// across bucket-by-bucket updates. BucketCount is always >=1 for any
	// entry present in the map (addToTagRollup only creates an entry when
	// adding a bucket to it), so this never divides by zero.
	for _, group := range result.Summary.TagRollup {
		group.AverageRiskScore = float64(group.RiskScore) / float64(group.BucketCount)
	}

	return result
}

// isDefaultKMSKey reports whether kmsKeyID refers to the AWS-managed default
// S3 KMS key (alias/aws/s3), as opposed to a customer-managed KMS key (CMK).
// The default key's ARN/alias always ends in "alias/aws/s3" regardless of
// account or region.
func isDefaultKMSKey(kmsKeyID string) bool {
	return strings.HasSuffix(kmsKeyID, "alias/aws/s3")
}

// addToTagRollup adds a single bucket's finding data to the summary's
// TagRollup, keyed by the value of the tagKey bucket tag (or "untagged" if
// the bucket has no such tag).
func addToTagRollup(summary *DiscoverySummary, tagKey string, info *s3.BucketInfo, discovery *BucketDiscovery) {
	if summary.TagRollup == nil {
		summary.TagRollup = make(map[string]*TagGroupSummary)
	}
	value := info.Tags[tagKey]
	if value == "" {
		value = untaggedGroupKey
	}
	group, ok := summary.TagRollup[value]
	if !ok {
		group = &TagGroupSummary{}
		summary.TagRollup[value] = group
	}
	group.BucketCount++
	group.RiskScore += discovery.RiskScore
	switch discovery.Status {
	case StatusUnusedBucket:
		group.UnusedCount++
	case StatusRisky:
		group.RiskyCount++
	case StatusInactive:
		group.InactiveCount++
	case StatusVersionSprawl:
		group.VersionSprawlCount++
	}
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

		if config.EstimateCost && info.TotalVersionSize > info.TotalSize {
			overhead := info.TotalVersionSize - info.TotalSize
			discovery.EstimatedMonthlyCostUSD = pricing.MonthlyStorageCost(overhead, info.Region)
		}

		if config.SuggestLifecyclePolicy {
			suggestion := remediation.SuggestLifecyclePolicy(info.Name)
			discovery.LifecyclePolicySuggestion = &suggestion
		}
	}

	// Factor 6: No encryption (40 points), or default-KMS-key granularity
	// (15 points) - if check enabled
	if config.CheckEncryption && info.Encryption != nil {
		if !info.Encryption.Enabled {
			discovery.RiskScore += 40
			discovery.RiskFactors = append(discovery.RiskFactors, "No encryption enabled")
			discovery.Recommendations = append(discovery.Recommendations,
				"Enable default encryption (AES256 or KMS)")
		} else if info.Encryption.Algorithm == "aws:kms" && isDefaultKMSKey(info.Encryption.KMSMasterKeyID) {
			discovery.RiskScore += 15
			discovery.RiskFactors = append(discovery.RiskFactors,
				"Encrypted with the default AWS-managed KMS key, not a customer-managed key")
			discovery.Recommendations = append(discovery.Recommendations,
				"Consider a customer-managed KMS key (CMK) if your compliance framework requires key rotation/access control the default key doesn't provide")
		}
	}

	// Factor 7: Public access (60 points - high risk, halved to 30 for
	// buckets matching a naming convention for intentionally-public content)
	// - if check enabled
	if config.CheckPublicAccess && info.PublicAccess != nil && info.PublicAccess.IsPublic {
		if IsAllowlistedPublicBucket(info.Name, config.PublicBucketAllowlistPatterns) {
			discovery.RiskScore += 30
			discovery.RiskFactors = append(discovery.RiskFactors,
				"Public access enabled (naming suggests intentional)")
			discovery.Recommendations = append(discovery.Recommendations,
				"Public access appears intentional based on naming convention; verify this is still correct")
		} else {
			discovery.RiskScore += 60
			discovery.RiskFactors = append(discovery.RiskFactors, "Public access enabled")
			discovery.Recommendations = append(discovery.Recommendations,
				"Review and restrict public access if not required")
		}
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

	// Price full bucket storage for Inactive/UnusedBucket findings -- the
	// more actionable "this has sat unused and costs you $X/month" figure,
	// distinct in scope from the version-sprawl overhead cost above. Guarded
	// on EstimatedMonthlyCostUSD == 0: the version-sprawl raw condition
	// (VersioningEnabled && LifecycleRules == 0) is independent of the final
	// Status classification above, so a bucket can be both empty/stale
	// (Status ends up Unused/Inactive, since that case is checked first in
	// the switch) AND version-sprawling (the Factor 5 cost block above still
	// ran). Without this guard both cost fields would populate for the same
	// bucket, contradicting CostUSD()'s "at most one populated" invariant.
	if config.EstimateCost && info.TotalSize > 0 && discovery.EstimatedMonthlyCostUSD == 0 &&
		(discovery.Status == StatusInactive || discovery.Status == StatusUnusedBucket) {
		discovery.EstimatedStorageCostUSD = pricing.MonthlyStorageCost(info.TotalSize, info.Region)
	}

	if managed {
		discovery.Recommendations = append([]string{managedBucketMessage}, discovery.Recommendations...)
	}

	return discovery
}
