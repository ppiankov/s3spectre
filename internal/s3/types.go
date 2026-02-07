package s3

import "time"

// BucketInfo contains metadata about an S3 bucket
type BucketInfo struct {
	Name              string            `json:"name"`
	Exists            bool              `json:"exists"`
	Region            string            `json:"region,omitempty"`
	CreationDate      *time.Time        `json:"creation_date,omitempty"`
	LastActivity      *time.Time        `json:"last_activity,omitempty"`
	DaysSinceActivity int               `json:"days_since_activity"`
	AgeInDays         int               `json:"age_in_days"`
	VersioningEnabled bool              `json:"versioning_enabled"`
	LifecycleRules    int               `json:"lifecycle_rules"`
	Prefixes          []PrefixInfo      `json:"prefixes,omitempty"`
	Tags              map[string]string `json:"tags,omitempty"`
	IsEmpty           bool              `json:"is_empty"`
	ObjectCount       int               `json:"object_count,omitempty"`
	TotalSize         int64             `json:"total_size,omitempty"`
	TotalVersionSize  int64             `json:"total_version_size,omitempty"`
	VersionCount      int               `json:"version_count,omitempty"`
	Encryption        *EncryptionInfo   `json:"encryption,omitempty"`
	PublicAccess      *PublicAccessInfo `json:"public_access,omitempty"`
	Error             string            `json:"error,omitempty"`
}

// EncryptionInfo contains bucket encryption configuration
type EncryptionInfo struct {
	Enabled        bool   `json:"enabled"`
	Algorithm      string `json:"algorithm,omitempty"` // AES256, aws:kms
	KMSMasterKeyID string `json:"kms_key_id,omitempty"`
}

// PublicAccessInfo contains public access block configuration
type PublicAccessInfo struct {
	IsPublic              bool `json:"is_public"`
	BlockPublicAcls       bool `json:"block_public_acls"`
	IgnorePublicAcls      bool `json:"ignore_public_acls"`
	BlockPublicPolicy     bool `json:"block_public_policy"`
	RestrictPublicBuckets bool `json:"restrict_public_buckets"`
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
