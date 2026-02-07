package analyzer

import (
	"fmt"
	"strings"

	"github.com/ppiankov/s3spectre/internal/s3"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

// Analyze analyzes the differences between code references and AWS S3 state
func Analyze(refs []scanner.Reference, bucketInfo map[string]*s3.BucketInfo, config Config) *Result {
	result := &Result{
		Buckets: make(map[string]*BucketAnalysis),
		Summary: Summary{},
	}

	// Build reference map
	referencedBuckets := make(map[string]bool)
	for _, ref := range refs {
		referencedBuckets[ref.Bucket] = true
	}

	// Analyze each bucket
	for bucket, info := range bucketInfo {
		analysis := analyzeBucket(bucket, info, refs, config, referencedBuckets)
		result.Buckets[bucket] = analysis

		// Update summary
		result.Summary.TotalBuckets++

		switch analysis.Status {
		case StatusOK:
			result.Summary.OKBuckets++
		case StatusMissingBucket:
			result.Summary.MissingBuckets = append(result.Summary.MissingBuckets, bucket)
		case StatusUnusedBucket:
			result.Summary.UnusedBuckets = append(result.Summary.UnusedBuckets, bucket)
		case StatusVersionSprawl:
			result.Summary.VersionSprawl = append(result.Summary.VersionSprawl, bucket)
		case StatusLifecycleMisconfig:
			result.Summary.LifecycleMisconfig = append(result.Summary.LifecycleMisconfig, bucket)
		}

		// Check prefix statuses
		for _, prefix := range analysis.Prefixes {
			prefixPath := fmt.Sprintf("%s/%s", bucket, prefix.Prefix)
			switch prefix.Status {
			case StatusMissingPrefix:
				result.Summary.MissingPrefixes = append(result.Summary.MissingPrefixes, prefixPath)
			case StatusStalePrefix:
				result.Summary.StalePrefixes = append(result.Summary.StalePrefixes, prefixPath)
			}
		}
	}

	return result
}

// analyzeBucket analyzes a single bucket
func analyzeBucket(bucket string, info *s3.BucketInfo, refs []scanner.Reference, config Config, referencedBuckets map[string]bool) *BucketAnalysis {
	analysis := &BucketAnalysis{
		Name:              bucket,
		ReferencedInCode:  referencedBuckets[bucket],
		ExistsInAWS:       info.Exists,
		VersioningEnabled: info.VersioningEnabled,
		LifecycleRules:    info.LifecycleRules,
	}

	// Check if bucket exists
	if !info.Exists {
		analysis.Status = StatusMissingBucket
		analysis.Message = "Bucket referenced in code but does not exist in AWS"
		return analysis
	}

	// Check for unused bucket if enabled
	if config.CheckUnused {
		unusedScore := calculateUnusedScore(bucket, info, referencedBuckets, config)
		analysis.UnusedScore = unusedScore

		if unusedScore.IsUnused {
			analysis.Status = StatusUnusedBucket
			analysis.Message = fmt.Sprintf("Bucket appears unused (score: %d/%d)", unusedScore.Total, config.UnusedScoreThreshold)
			return analysis
		}
	}

	// Check version sprawl (versioning enabled but no lifecycle rules)
	if info.VersioningEnabled && info.LifecycleRules == 0 {
		analysis.Status = StatusVersionSprawl
		analysis.Message = "Versioning enabled but no lifecycle rules to clean up old versions"
		// Don't return yet, still check prefixes
	}

	// Analyze prefixes
	bucketRefs := filterRefsByBucket(refs, bucket)
	if len(info.Prefixes) > 0 {
		analysis.Prefixes = analyzePrefixes(info.Prefixes, bucketRefs, config)
	}

	// Determine overall status if not already set
	if analysis.Status == "" {
		// Check for lifecycle misconfig (heuristic: no lifecycle rules for large buckets)
		if info.LifecycleRules == 0 && len(info.Prefixes) > 0 {
			hasLargePrefix := false
			for _, p := range info.Prefixes {
				if p.ObjectCount > 100 {
					hasLargePrefix = true
					break
				}
			}
			if hasLargePrefix {
				analysis.Status = StatusLifecycleMisconfig
				analysis.Message = "Bucket has no lifecycle rules but contains many objects"
			}
		}
	}

	// Default to OK if no issues found
	if analysis.Status == "" {
		analysis.Status = StatusOK
		analysis.Message = "Bucket exists and matches expected usage"
	}

	return analysis
}

// calculateUnusedScore calculates a score to determine if a bucket is unused
func calculateUnusedScore(bucket string, info *s3.BucketInfo, referencedBuckets map[string]bool, config Config) *UnusedScore {
	score := &UnusedScore{
		Total:   0,
		Reasons: make([]string, 0),
	}

	// Threshold for scoring (default 150 points = unused)
	threshold := config.UnusedScoreThreshold
	if threshold <= 0 {
		threshold = 150
	}

	// Score: Not referenced in code (100 points)
	if !referencedBuckets[bucket] {
		score.NotInCode = 100
		score.Total += 100
		score.Reasons = append(score.Reasons, "Not referenced in code")
	}

	// Score: Bucket is empty (50 points)
	if info.IsEmpty {
		score.Empty = 50
		score.Total += 50
		score.Reasons = append(score.Reasons, "Bucket is empty")
	}

	// Score: Bucket is old (creation date check - we don't have this yet, so check tags or skip)
	// For now, we'll skip this since we don't have creation date

	// Score: Has deprecated/old tags (20 points)
	if info.Tags != nil {
		deprecatedTags := []string{"deprecated", "old", "unused", "delete", "obsolete", "legacy"}
		for key, value := range info.Tags {
			keyLower := strings.ToLower(key)
			valueLower := strings.ToLower(value)
			for _, deprecated := range deprecatedTags {
				if keyLower == deprecated || valueLower == deprecated {
					score.DeprecatedTag = 20
					score.Total += 20
					score.Reasons = append(score.Reasons, fmt.Sprintf("Has deprecated tag: %s=%s", key, value))
					break
				}
			}
			if score.DeprecatedTag > 0 {
				break
			}
		}
	}

	// Determine if unused
	score.IsUnused = score.Total >= threshold

	return score
}

// analyzePrefixes analyzes prefixes for a bucket
func analyzePrefixes(prefixes []s3.PrefixInfo, refs []scanner.Reference, config Config) []PrefixAnalysis {
	var results []PrefixAnalysis

	for _, prefix := range prefixes {
		analysis := PrefixAnalysis{
			Prefix:            prefix.Prefix,
			ObjectCount:       prefix.ObjectCount,
			DaysSinceModified: prefix.DaysSinceModified,
		}

		if !prefix.Exists {
			analysis.Status = StatusMissingPrefix
			analysis.Message = "Prefix referenced in code but no objects found"
		} else if prefix.DaysSinceModified > config.StaleThresholdDays {
			analysis.Status = StatusStalePrefix
			analysis.Message = fmt.Sprintf("No modifications for %d days (threshold: %d)",
				prefix.DaysSinceModified, config.StaleThresholdDays)
		} else {
			analysis.Status = StatusOK
		}

		results = append(results, analysis)
	}

	return results
}

// filterRefsByBucket filters references for a specific bucket
func filterRefsByBucket(refs []scanner.Reference, bucket string) []scanner.Reference {
	var filtered []scanner.Reference
	for _, ref := range refs {
		if ref.Bucket == bucket {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}
