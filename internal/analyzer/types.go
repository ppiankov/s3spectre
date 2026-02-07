package analyzer

// Status represents the status of a bucket/prefix
type Status string

const (
	StatusOK                 Status = "OK"
	StatusMissingBucket      Status = "MISSING_BUCKET"
	StatusUnusedBucket       Status = "UNUSED_BUCKET"
	StatusMissingPrefix      Status = "MISSING_PREFIX"
	StatusStalePrefix        Status = "STALE_PREFIX"
	StatusVersionSprawl      Status = "VERSION_SPRAWL"
	StatusLifecycleMisconfig Status = "LIFECYCLE_MISCONFIG"
	StatusRisky              Status = "RISKY"
	StatusInactive           Status = "INACTIVE"
)

// BucketAnalysis contains analysis results for a bucket
type BucketAnalysis struct {
	Name              string        `json:"name"`
	Status            Status        `json:"status"`
	Message           string        `json:"message,omitempty"`
	ReferencedInCode  bool          `json:"referenced_in_code"`
	ExistsInAWS       bool          `json:"exists_in_aws"`
	VersioningEnabled bool          `json:"versioning_enabled"`
	LifecycleRules    int           `json:"lifecycle_rules"`
	Prefixes          []PrefixAnalysis `json:"prefixes,omitempty"`
	UnusedScore       *UnusedScore  `json:"unused_score,omitempty"`
}

// PrefixAnalysis contains analysis results for a prefix
type PrefixAnalysis struct {
	Prefix            string `json:"prefix"`
	Status            Status `json:"status"`
	Message           string `json:"message,omitempty"`
	ObjectCount       int    `json:"object_count"`
	DaysSinceModified int    `json:"days_since_modified,omitempty"`
}

// Summary contains high-level analysis summary
type Summary struct {
	TotalBuckets         int      `json:"total_buckets"`
	OKBuckets            int      `json:"ok_buckets"`
	MissingBuckets       []string `json:"missing_buckets,omitempty"`
	UnusedBuckets        []string `json:"unused_buckets,omitempty"`
	MissingPrefixes      []string `json:"missing_prefixes,omitempty"`
	StalePrefixes        []string `json:"stale_prefixes,omitempty"`
	VersionSprawl        []string `json:"version_sprawl,omitempty"`
	LifecycleMisconfig   []string `json:"lifecycle_misconfig,omitempty"`
}

// Result contains the complete analysis result
type Result struct {
	Summary Summary                    `json:"summary"`
	Buckets map[string]*BucketAnalysis `json:"buckets"`
}

// Config contains analyzer configuration
type Config struct {
	StaleThresholdDays    int
	UnusedThresholdDays   int
	CheckUnused           bool
	UnusedScoreThreshold  int
}

// UnusedScore contains scoring details for unused bucket detection
type UnusedScore struct {
	Total          int      `json:"total"`
	Reasons        []string `json:"reasons"`
	IsUnused       bool     `json:"is_unused"`
	NotInCode      int      `json:"not_in_code"`
	Empty          int      `json:"empty"`
	OldBucket      int      `json:"old_bucket"`
	DeprecatedTag  int      `json:"deprecated_tag"`
}
