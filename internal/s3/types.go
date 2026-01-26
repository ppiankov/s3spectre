package s3

import "time"

// BucketInfo contains metadata about an S3 bucket
type BucketInfo struct {
	Name              string            `json:"name"`
	Exists            bool              `json:"exists"`
	Region            string            `json:"region,omitempty"`
	CreationDate      *time.Time        `json:"creation_date,omitempty"`
	VersioningEnabled bool              `json:"versioning_enabled"`
	LifecycleRules    int               `json:"lifecycle_rules"`
	Prefixes          []PrefixInfo      `json:"prefixes,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`
	IsEmpty           bool              `json:"is_empty"`
	Error             string            `json:"error,omitempty"`
}

// PrefixInfo contains metadata about an S3 prefix
type PrefixInfo struct {
	Prefix           string     `json:"prefix"`
	Exists           bool       `json:"exists"`
	ObjectCount      int        `json:"object_count"`
	LatestModified   *time.Time `json:"latest_modified,omitempty"`
	TotalVersions    int        `json:"total_versions,omitempty"`
	DaysSinceModified int       `json:"days_since_modified,omitempty"`
}
