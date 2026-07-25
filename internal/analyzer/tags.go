package analyzer

import "strings"

// deprecatedTagMarkers lists the case-insensitive tag keys/values that mark a
// bucket as deprecated, unused, or scheduled for removal. Shared by scan
// (calculateUnusedScore) and discover (analyzeBucketDiscovery) so both
// commands agree on what counts as a deprecated tag.
var deprecatedTagMarkers = []string{"deprecated", "old", "unused", "delete", "obsolete", "legacy", "retired"}

// IsDeprecatedTag reports whether any tag key or value matches a known
// deprecated-tag marker, and returns the first matching key/value pair.
func IsDeprecatedTag(tags map[string]string) (matched bool, key string, value string) {
	if tags == nil {
		return false, "", ""
	}
	for k, v := range tags {
		kLower := strings.ToLower(k)
		vLower := strings.ToLower(v)
		for _, marker := range deprecatedTagMarkers {
			if kLower == marker || vLower == marker {
				return true, k, v
			}
		}
	}
	return false, "", ""
}
