package analyzer

import "strings"

// defaultPublicBucketAllowlistPatterns lists common naming-convention
// substrings for buckets that are intentionally public by design (static
// web content, app config CDNs, landing pages). A bucket matching one of
// these still gets flagged as public -- it is never silently dropped --
// but at reduced severity, so an operator's attention is drawn to the
// smaller set of buckets where public access is unexplained by naming.
var defaultPublicBucketAllowlistPatterns = []string{
	"public",
	"webview",
	"-cdn",
	"-landing",
}

// IsAllowlistedPublicBucket reports whether name matches a built-in or
// operator-supplied naming pattern for intentionally-public buckets.
// Matching is a case-insensitive substring test against both the default
// patterns and any extraPatterns (additive, not a replacement).
func IsAllowlistedPublicBucket(name string, extraPatterns []string) bool {
	lower := strings.ToLower(name)
	for _, p := range defaultPublicBucketAllowlistPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	for _, p := range extraPatterns {
		if p == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
