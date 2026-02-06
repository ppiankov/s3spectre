package report

import (
	"time"

	"github.com/ppiankov/s3spectre/internal/analyzer"
)

// DiscoveryData contains discovery report data
type DiscoveryData struct {
	Tool      string                           `json:"tool"`
	Version   string                           `json:"version"`
	Timestamp time.Time                        `json:"timestamp"`
	Config    DiscoveryConfig                  `json:"config"`
	Summary   analyzer.DiscoverySummary         `json:"summary"`
	Buckets   map[string]*analyzer.BucketDiscovery `json:"buckets"`
}

// DiscoveryConfig contains discovery scan configuration
type DiscoveryConfig struct {
	AWSProfile              string   `json:"aws_profile,omitempty"`
	AllRegions              bool     `json:"all_regions"`
	Regions                 []string `json:"regions,omitempty"`
	AgeThresholdDays        int      `json:"age_threshold_days"`
	InactivityThresholdDays int      `json:"inactivity_threshold_days"`
	CheckEncryption         bool     `json:"check_encryption"`
	CheckPublicAccess       bool     `json:"check_public_access"`
}
