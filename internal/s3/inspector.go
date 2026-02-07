package s3

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

// ProgressCallback is called during inspection to report progress
type ProgressCallback func(current, total int, message string)

// Inspector inspects S3 buckets and prefixes
type Inspector struct {
	client           *Client
	concurrency      int
	progressCallback ProgressCallback
	regions          []string
	allRegions       bool
}

// NewInspector creates a new S3 inspector
func NewInspector(client *Client, concurrency int) *Inspector {
	if concurrency <= 0 {
		concurrency = 10
	}
	return &Inspector{
		client:      client,
		concurrency: concurrency,
		allRegions:  false,
	}
}

// SetProgressCallback sets the progress callback function
func (i *Inspector) SetProgressCallback(callback ProgressCallback) {
	i.progressCallback = callback
}

// SetRegions sets specific regions to scan
func (i *Inspector) SetRegions(regions []string) {
	i.regions = regions
	i.allRegions = false
}

// SetAllRegions enables scanning all AWS regions
func (i *Inspector) SetAllRegions(enabled bool) {
	i.allRegions = enabled
}

// reportProgress calls the progress callback if set
func (i *Inspector) reportProgress(current, total int, message string) {
	if i.progressCallback != nil {
		i.progressCallback(current, total, message)
	}
}

// InspectBuckets inspects all buckets referenced in the code
func (i *Inspector) InspectBuckets(ctx context.Context, refs []scanner.Reference) (map[string]*BucketInfo, error) {
	// Determine which regions to scan
	regions, err := i.determineRegions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine regions: %w", err)
	}

	i.reportProgress(0, 1, fmt.Sprintf("Scanning %d region(s)", len(regions)))

	// Group references by bucket
	bucketRefs := make(map[string][]scanner.Reference)
	for _, ref := range refs {
		bucketRefs[ref.Bucket] = append(bucketRefs[ref.Bucket], ref)
	}

	// Fetch all AWS buckets across all regions
	i.reportProgress(0, 2, "Listing buckets across regions")
	awsBuckets, bucketRegions, err := i.listAllBucketsMultiRegion(ctx, regions)
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS buckets: %w", err)
	}

	// Create bucket info map
	bucketInfo := make(map[string]*BucketInfo)
	for bucket := range bucketRefs {
		bucketInfo[bucket] = &BucketInfo{
			Name:   bucket,
			Exists: awsBuckets[bucket],
			Region: bucketRegions[bucket],
		}
	}

	// Inspect buckets concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, i.concurrency)

	total := len(bucketRefs)
	current := 0

	for bucket, refs := range bucketRefs {
		wg.Add(1)
		go func(bucket string, refs []scanner.Reference) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			info := i.inspectBucket(ctx, bucket, refs)

			mu.Lock()
			current++
			i.reportProgress(current, total, fmt.Sprintf("Inspecting bucket %s", bucket))
			bucketInfo[bucket] = info
			mu.Unlock()
		}(bucket, refs)
	}

	wg.Wait()

	return bucketInfo, nil
}

// determineRegions determines which regions to scan based on configuration
func (i *Inspector) determineRegions(ctx context.Context) ([]string, error) {
	// If specific regions are set, use those
	if len(i.regions) > 0 {
		return i.regions, nil
	}

	// If all regions mode is enabled, list all regions
	if i.allRegions {
		regions, err := i.client.ListRegions(ctx)
		if err != nil {
			return nil, err
		}
		return regions, nil
	}

	// Default to the client's configured region
	return []string{i.client.GetRegion()}, nil
}

// listAllBucketsMultiRegion lists all buckets and determines their regions
func (i *Inspector) listAllBucketsMultiRegion(ctx context.Context, regions []string) (map[string]bool, map[string]string, error) {
	buckets := make(map[string]bool)
	bucketRegions := make(map[string]string)

	// ListBuckets returns all buckets regardless of region, so we only need to call it once
	var result *s3.ListBucketsOutput
	err := i.client.WithRetry(ctx, func() error {
		var err error
		result, err = i.client.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
		return err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	// For each bucket, determine its region
	for _, bucket := range result.Buckets {
		if bucket.Name != nil {
			bucketName := *bucket.Name
			buckets[bucketName] = true

			// Get bucket region
			region, err := i.getBucketRegion(ctx, bucketName)
			if err != nil {
				// If we can't determine the region, use the default
				region = i.client.GetRegion()
			}
			bucketRegions[bucketName] = region
		}
	}

	return buckets, bucketRegions, nil
}

// getBucketRegion gets the region of a specific bucket
func (i *Inspector) getBucketRegion(ctx context.Context, bucket string) (string, error) {
	var locationResult *s3.GetBucketLocationOutput
	err := i.client.WithRetry(ctx, func() error {
		var err error
		locationResult, err = i.client.s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
			Bucket: aws.String(bucket),
		})
		return err
	})
	if err != nil {
		return "", err
	}

	// Handle the special case where us-east-1 returns empty string
	if locationResult.LocationConstraint == "" {
		return "us-east-1", nil
	}

	return string(locationResult.LocationConstraint), nil
}

// inspectBucket inspects a single bucket
func (i *Inspector) inspectBucket(ctx context.Context, bucket string, refs []scanner.Reference) *BucketInfo {
	info := &BucketInfo{
		Name:   bucket,
		Exists: false,
	}

	// Get bucket region first
	region, err := i.getBucketRegion(ctx, bucket)
	if err != nil {
		info.Error = formatError("get bucket location", bucket, err)
		return info
	}

	info.Exists = true
	info.Region = region

	// Create region-specific client if needed
	regionClient := i.client
	if region != i.client.GetRegion() {
		regionClient = NewClientForRegion(i.client.GetConfig(), region)
	}

	// Get bucket creation date (from ListBuckets - we'll get it from the bucket metadata)
	// Note: GetBucketLocation doesn't return creation date, we'd need to call ListBuckets
	// For efficiency, we'll skip this for now or get it from tags

	// Get versioning status
	err = regionClient.WithRetry(ctx, func() error {
		versioningResult, err := regionClient.s3Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
			Bucket: aws.String(bucket),
		})
		if err == nil {
			info.VersioningEnabled = versioningResult.Status == types.BucketVersioningStatusEnabled
		}
		return err
	})
	if err != nil {
		// Non-fatal, continue
		info.Error = formatError("get versioning", bucket, err)
	}

	// Get lifecycle configuration
	_ = regionClient.WithRetry(ctx, func() error {
		lifecycleResult, err := regionClient.s3Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
			Bucket: aws.String(bucket),
		})
		if err == nil && lifecycleResult.Rules != nil {
			info.LifecycleRules = len(lifecycleResult.Rules)
		}
		// Lifecycle not existing is not an error
		if err != nil && strings.Contains(err.Error(), "NoSuchLifecycleConfiguration") {
			return nil
		}
		return err
	})
	// Non-fatal error, continue

	// Get bucket tagging for unused detection
	_ = regionClient.WithRetry(ctx, func() error {
		taggingResult, err := regionClient.s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
			Bucket: aws.String(bucket),
		})
		if err == nil && taggingResult.TagSet != nil {
			info.Tags = make(map[string]string)
			for _, tag := range taggingResult.TagSet {
				if tag.Key != nil && tag.Value != nil {
					info.Tags[*tag.Key] = *tag.Value
				}
			}
		}
		// No tags is not an error
		if err != nil && strings.Contains(err.Error(), "NoSuchTagSet") {
			return nil
		}
		return err
	})
	// Non-fatal error, continue

	// Check if bucket is empty (for unused detection)
	_ = regionClient.WithRetry(ctx, func() error {
		listResult, err := regionClient.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket),
			MaxKeys: aws.Int32(1),
		})
		if err == nil {
			info.IsEmpty = listResult.KeyCount != nil && *listResult.KeyCount == 0
		}
		return err
	})
	// Non-fatal error, continue

	// Inspect prefixes
	prefixes := i.extractPrefixes(refs)
	if len(prefixes) > 0 {
		info.Prefixes = i.inspectPrefixesWithClient(ctx, regionClient, bucket, prefixes)
	}

	return info
}

// formatError formats an error message with context
func formatError(operation, resource string, err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Provide helpful context for common errors
	if strings.Contains(errMsg, "AccessDenied") || strings.Contains(errMsg, "Access Denied") {
		return fmt.Sprintf("%s failed for %s: Access Denied - check IAM permissions", operation, resource)
	}
	if strings.Contains(errMsg, "NoSuchBucket") {
		return fmt.Sprintf("%s failed for %s: Bucket does not exist or is in a different region", operation, resource)
	}
	if strings.Contains(errMsg, "RequestLimitExceeded") || strings.Contains(errMsg, "SlowDown") {
		return fmt.Sprintf("%s failed for %s: Rate limit exceeded - consider reducing --concurrency", operation, resource)
	}

	return fmt.Sprintf("%s failed for %s: %s", operation, resource, errMsg)
}

// extractPrefixes extracts unique prefixes from references
func (i *Inspector) extractPrefixes(refs []scanner.Reference) []string {
	prefixMap := make(map[string]bool)
	var prefixes []string

	for _, ref := range refs {
		if ref.Prefix != "" {
			if !prefixMap[ref.Prefix] {
				prefixes = append(prefixes, ref.Prefix)
				prefixMap[ref.Prefix] = true
			}
		}
	}

	return prefixes
}

// inspectPrefixesWithClient inspects multiple prefixes using a specific client
func (i *Inspector) inspectPrefixesWithClient(ctx context.Context, client *Client, bucket string, prefixes []string) []PrefixInfo {
	var results []PrefixInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, i.concurrency)

	for _, prefix := range prefixes {
		wg.Add(1)
		go func(prefix string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			info := i.inspectPrefixWithClient(ctx, client, bucket, prefix)

			mu.Lock()
			results = append(results, info)
			mu.Unlock()
		}(prefix)
	}

	wg.Wait()

	return results
}

// inspectPrefixWithClient inspects a single prefix using a specific client
func (i *Inspector) inspectPrefixWithClient(ctx context.Context, client *Client, bucket, prefix string) PrefixInfo {
	info := PrefixInfo{
		Prefix: prefix,
		Exists: false,
	}

	// List objects with this prefix (limited to 1000 for MVP)
	var listResult *s3.ListObjectsV2Output
	err := client.WithRetry(ctx, func() error {
		var err error
		listResult, err = client.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket),
			Prefix:  aws.String(prefix),
			MaxKeys: aws.Int32(1000),
		})
		return err
	})
	if err != nil {
		return info
	}

	if listResult.KeyCount == nil || *listResult.KeyCount == 0 {
		return info
	}

	info.Exists = true
	info.ObjectCount = int(*listResult.KeyCount)

	// Find latest modified
	var latest *time.Time
	for _, obj := range listResult.Contents {
		if obj.LastModified != nil {
			if latest == nil || obj.LastModified.After(*latest) {
				latest = obj.LastModified
			}
		}
	}

	if latest != nil {
		info.LatestModified = latest
		info.DaysSinceModified = int(time.Since(*latest).Hours() / 24)
	}

	return info
}

// DiscoverAllBuckets discovers and inspects all S3 buckets in the account without code references
func (i *Inspector) DiscoverAllBuckets(ctx context.Context) (map[string]*BucketInfo, error) {
	// Determine regions
	regions, err := i.determineRegions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine regions: %w", err)
	}

	i.reportProgress(0, 1, fmt.Sprintf("Discovering buckets across %d region(s)", len(regions)))

	// List ALL buckets and their regions
	i.reportProgress(0, 2, "Listing all S3 buckets")
	awsBuckets, bucketRegions, bucketMetadata, err := i.listAllBucketsWithMetadata(ctx, regions)
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS buckets: %w", err)
	}

	// Inspect each bucket
	bucketInfo := make(map[string]*BucketInfo)
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, i.concurrency)

	total := len(awsBuckets)
	current := 0

	for bucketName := range awsBuckets {
		wg.Add(1)
		go func(bucket string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			region := bucketRegions[bucket]
			metadata := bucketMetadata[bucket]
			info := i.inspectBucketFull(ctx, bucket, region, metadata)

			mu.Lock()
			current++
			i.reportProgress(current, total, fmt.Sprintf("Inspecting %s", bucket))
			bucketInfo[bucket] = info
			mu.Unlock()
		}(bucketName)
	}

	wg.Wait()

	return bucketInfo, nil
}

// bucketMetadata holds bucket-level metadata from ListBuckets
type bucketMetadata struct {
	CreationDate *time.Time
}

// listAllBucketsWithMetadata lists all buckets with their metadata
func (i *Inspector) listAllBucketsWithMetadata(ctx context.Context, regions []string) (map[string]bool, map[string]string, map[string]*bucketMetadata, error) {
	buckets := make(map[string]bool)
	bucketRegions := make(map[string]string)
	metadata := make(map[string]*bucketMetadata)

	// ListBuckets returns all buckets regardless of region
	var result *s3.ListBucketsOutput
	err := i.client.WithRetry(ctx, func() error {
		var err error
		result, err = i.client.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
		return err
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	// For each bucket, store metadata and determine region
	for _, bucket := range result.Buckets {
		if bucket.Name != nil {
			bucketName := *bucket.Name
			buckets[bucketName] = true

			// Store creation date
			metadata[bucketName] = &bucketMetadata{
				CreationDate: bucket.CreationDate,
			}

			// Get bucket region
			region, err := i.getBucketRegion(ctx, bucketName)
			if err != nil {
				region = i.client.GetRegion() // Fallback to default
			}
			bucketRegions[bucketName] = region
		}
	}

	return buckets, bucketRegions, metadata, nil
}

// inspectBucketFull performs full inspection without needing code references
func (i *Inspector) inspectBucketFull(ctx context.Context, bucket, region string, metadata *bucketMetadata) *BucketInfo {
	info := &BucketInfo{
		Name:   bucket,
		Region: region,
		Exists: true,
	}

	// Set creation date and age
	if metadata != nil && metadata.CreationDate != nil {
		info.CreationDate = metadata.CreationDate
		info.AgeInDays = int(time.Since(*metadata.CreationDate).Hours() / 24)
	}

	// Create region-specific client if needed
	regionClient := i.client
	if region != i.client.GetRegion() {
		regionClient = NewClientForRegion(i.client.GetConfig(), region)
	}

	// Get versioning status
	_ = regionClient.WithRetry(ctx, func() error {
		versioningResult, err := regionClient.s3Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
			Bucket: aws.String(bucket),
		})
		if err == nil {
			info.VersioningEnabled = versioningResult.Status == types.BucketVersioningStatusEnabled
		}
		return err
	})

	// Get lifecycle configuration
	_ = regionClient.WithRetry(ctx, func() error {
		lifecycleResult, err := regionClient.s3Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
			Bucket: aws.String(bucket),
		})
		if err == nil && lifecycleResult.Rules != nil {
			info.LifecycleRules = len(lifecycleResult.Rules)
		}
		if err != nil && strings.Contains(err.Error(), "NoSuchLifecycleConfiguration") {
			return nil
		}
		return err
	})

	// Get bucket tagging
	_ = regionClient.WithRetry(ctx, func() error {
		taggingResult, err := regionClient.s3Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
			Bucket: aws.String(bucket),
		})
		if err == nil && taggingResult.TagSet != nil {
			info.Tags = make(map[string]string)
			for _, tag := range taggingResult.TagSet {
				if tag.Key != nil && tag.Value != nil {
					info.Tags[*tag.Key] = *tag.Value
				}
			}
		}
		if err != nil && strings.Contains(err.Error(), "NoSuchTagSet") {
			return nil
		}
		return err
	})

	// Check if empty and get last activity
	_ = regionClient.WithRetry(ctx, func() error {
		listResult, err := regionClient.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:  aws.String(bucket),
			MaxKeys: aws.Int32(100), // Sample first 100 objects
		})
		if err == nil {
			if listResult.KeyCount != nil {
				info.IsEmpty = *listResult.KeyCount == 0
				info.ObjectCount = int(*listResult.KeyCount)

				// Calculate size and find most recent object modification
				var latest *time.Time
				var totalSize int64
				for _, obj := range listResult.Contents {
					if obj.Size != nil {
						totalSize += *obj.Size
					}
					if obj.LastModified != nil {
						if latest == nil || obj.LastModified.After(*latest) {
							latest = obj.LastModified
						}
					}
				}

				info.TotalSize = totalSize

				if latest != nil {
					info.LastActivity = latest
					info.DaysSinceActivity = int(time.Since(*latest).Hours() / 24)
				}
			}
		}
		return err
	})

	// For versioned buckets, calculate total version size and count
	if info.VersioningEnabled {
		i.calculateVersionSizes(ctx, regionClient, bucket, info)
	}

	return info
}

// calculateVersionSizes calculates total size of all versions in a bucket
func (i *Inspector) calculateVersionSizes(ctx context.Context, client *Client, bucket string, info *BucketInfo) {
	var totalVersionSize int64
	var versionCount int
	var keyMarker *string
	var versionIDMarker *string

	maxIterations := 100
	iteration := 0

	_ = client.WithRetry(ctx, func() error {
		for iteration < maxIterations {
			listVersionsResult, err := client.s3Client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
				Bucket:          aws.String(bucket),
				MaxKeys:         aws.Int32(1000),
				KeyMarker:       keyMarker,
				VersionIdMarker: versionIDMarker,
			})

			if err != nil {
				return err
			}

			if listVersionsResult.Versions != nil {
				for _, version := range listVersionsResult.Versions {
					if version.Size != nil {
						totalVersionSize += *version.Size
					}
					versionCount++
				}
			}

			if listVersionsResult.DeleteMarkers != nil {
				versionCount += len(listVersionsResult.DeleteMarkers)
			}

			if listVersionsResult.IsTruncated != nil && *listVersionsResult.IsTruncated {
				keyMarker = listVersionsResult.NextKeyMarker
				versionIDMarker = listVersionsResult.NextVersionIdMarker
				iteration++
			} else {
				break
			}
		}
		return nil
	})

	info.TotalVersionSize = totalVersionSize
	info.VersionCount = versionCount
}
