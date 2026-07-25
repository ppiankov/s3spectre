package analyzer

import (
	"testing"

	"github.com/ppiankov/s3spectre/internal/s3"
)

func TestIsDeprecatedTag(t *testing.T) {
	tests := []struct {
		name     string
		tags     map[string]string
		expected bool
	}{
		{"nil tags", nil, false},
		{"empty tags", map[string]string{}, false},
		{"no deprecated tags", map[string]string{"env": "prod"}, false},
		{"deprecated key", map[string]string{"deprecated": "true"}, true},
		{"deprecated value", map[string]string{"status": "deprecated"}, true},
		{"case insensitive key", map[string]string{"DEPRECATED": "yes"}, true},
		{"case insensitive value", map[string]string{"status": "OBSOLETE"}, true},
		{"old tag", map[string]string{"old": "yes"}, true},
		{"unused tag", map[string]string{"unused": "true"}, true},
		{"delete tag", map[string]string{"delete": "true"}, true},
		{"legacy tag", map[string]string{"legacy": "true"}, true},
		{"retired tag", map[string]string{"retired": "true"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _ := IsDeprecatedTag(tt.tags)
			if got != tt.expected {
				t.Errorf("IsDeprecatedTag(%v) = %v, want %v", tt.tags, got, tt.expected)
			}
		})
	}
}

// TestScanDiscoverAgreeOnRetiredTag guards against the historical drift where
// calculateUnusedScore's local deprecated-tag list omitted "retired" while
// analyzeBucketDiscovery's included it, so a retired-tagged bucket was flagged
// unused by discover but not by scan.
func TestScanDiscoverAgreeOnRetiredTag(t *testing.T) {
	tags := map[string]string{"lifecycle": "retired"}
	info := &s3.BucketInfo{Name: "test-bucket", Exists: true, Tags: tags}

	scanScore := calculateUnusedScore("test-bucket", info, map[string]bool{"test-bucket": true}, Config{})
	if scanScore.DeprecatedTag == 0 {
		t.Errorf("expected scan (calculateUnusedScore) to flag a retired-tagged bucket, got score %+v", scanScore)
	}

	discovered, _, _ := IsDeprecatedTag(tags)
	if !discovered {
		t.Errorf("expected discover path (IsDeprecatedTag) to flag a retired-tagged bucket")
	}
}
