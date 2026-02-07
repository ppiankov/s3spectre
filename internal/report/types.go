package report

import (
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

// Reporter interface for different report formats
type Reporter interface {
	Generate(data Data) error
	GenerateDiscovery(data DiscoveryData) error
}

// Data contains all report data
type Data struct {
	Tool       string                         `json:"tool"`
	Version    string                         `json:"version"`
	Timestamp  time.Time                      `json:"timestamp"`
	Config     Config                         `json:"config"`
	Summary    analyzer.Summary               `json:"summary"`
	Buckets    map[string]*analyzer.BucketAnalysis `json:"buckets"`
	References []scanner.Reference            `json:"references,omitempty"`
}

// Config contains scan configuration
type Config struct {
	RepoPath           string `json:"repo_path"`
	AWSProfile         string `json:"aws_profile,omitempty"`
	AWSRegion          string `json:"aws_region,omitempty"`
	StaleThresholdDays int    `json:"stale_threshold_days"`
}
