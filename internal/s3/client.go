package s3

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps the AWS S3 client
type Client struct {
	s3Client *s3.Client
	config   aws.Config
}

// NewClient creates a new S3 client
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	// Load AWS config
	opts := []func(*config.LoadOptions) error{}

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		s3Client: s3.NewFromConfig(cfg),
		config:   cfg,
	}, nil
}

// GetClient returns the underlying S3 client
func (c *Client) GetClient() *s3.Client {
	return c.s3Client
}

// GetRegion returns the configured region
func (c *Client) GetRegion() string {
	return c.config.Region
}

// GetConfig returns the AWS config
func (c *Client) GetConfig() aws.Config {
	return c.config
}

// ListRegions returns all enabled AWS regions
func (c *Client) ListRegions(ctx context.Context) ([]string, error) {
	// Create EC2 client to list regions
	ec2Client := ec2.NewFromConfig(c.config)

	result, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(false), // Only enabled regions
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS regions: %w", err)
	}

	regions := make([]string, 0, len(result.Regions))
	for _, region := range result.Regions {
		if region.RegionName != nil {
			regions = append(regions, *region.RegionName)
		}
	}

	return regions, nil
}

// NewClientForRegion creates a new S3 client for a specific region
func NewClientForRegion(baseConfig aws.Config, region string) *Client {
	// Create a new config with the specified region
	cfg := baseConfig.Copy()
	cfg.Region = region

	return &Client{
		s3Client: s3.NewFromConfig(cfg),
		config:   cfg,
	}
}

// WithRetry wraps an S3 operation with retry logic for transient errors
func (c *Client) WithRetry(ctx context.Context, operation func() error) error {
	const maxRetries = 3
	const baseDelay = 1 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return err
		}

		lastErr = err

		// Don't sleep on the last attempt
		if attempt < maxRetries-1 {
			// Exponential backoff with jitter
			delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Check for common retryable errors
	retryableErrors := []string{
		"RequestLimitExceeded",
		"ServiceUnavailable",
		"SlowDown",
		"RequestTimeout",
		"TooManyRequests",
		"InternalError",
		"503",
		"429",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}
