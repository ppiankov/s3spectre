package scanner

// Reference represents an S3 bucket/prefix reference found in code
type Reference struct {
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix,omitempty"`
	VersionID string `json:"version_id,omitempty"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Context   string `json:"context,omitempty"` // e.g., "read", "write", "list"
}

// RefType represents the type of S3 operation
type RefType string

const (
	RefTypeRead   RefType = "read"
	RefTypeWrite  RefType = "write"
	RefTypeList   RefType = "list"
	RefTypeUnknown RefType = "unknown"
)
