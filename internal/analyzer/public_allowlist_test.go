package analyzer

import "testing"

func TestIsAllowlistedPublicBucket_DefaultPatterns(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"acme-apk-public-prod", true},
		{"acme-prod-web-public", true},
		{"acme-int-webview", true},
		{"acme-prod-mobile-config-cdn", true},
		{"acme-prod-landing", true},
		{"loyalty-public-prod", true},
		{"internal-recommendation-service", false},
		{"acme-ios-certs", false},
		{"acme-secrets-vault", false},
	}

	for _, tt := range cases {
		if got := IsAllowlistedPublicBucket(tt.name, nil); got != tt.want {
			t.Errorf("IsAllowlistedPublicBucket(%q, nil) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsAllowlistedPublicBucket_CaseInsensitive(t *testing.T) {
	if !IsAllowlistedPublicBucket("MyCompany-WebView-Prod", nil) {
		t.Error("expected case-insensitive match for a default pattern")
	}
}

// TestIsAllowlistedPublicBucket_ConfigPatternsAreAdditive guards against a
// config-supplied allowlist pattern silently replacing the built-in
// defaults instead of extending them.
func TestIsAllowlistedPublicBucket_ConfigPatternsAreAdditive(t *testing.T) {
	extra := []string{"my-custom-suffix"}

	if !IsAllowlistedPublicBucket("some-bucket-my-custom-suffix", extra) {
		t.Error("expected config-supplied pattern to match")
	}
	// A default pattern must still match even when extra patterns are supplied.
	if !IsAllowlistedPublicBucket("acme-apk-public-prod", extra) {
		t.Error("expected built-in default pattern to still match alongside config patterns")
	}
}

func TestIsAllowlistedPublicBucket_NonMatchingBucketUnaffected(t *testing.T) {
	if IsAllowlistedPublicBucket("internal-recommendation-service", []string{"my-custom-suffix"}) {
		t.Error("expected a bucket matching no pattern (default or config) to not be allowlisted")
	}
}

func TestIsAllowlistedPublicBucket_EmptyPatternIgnored(t *testing.T) {
	// An empty string in extraPatterns must not match everything.
	if IsAllowlistedPublicBucket("internal-recommendation-service", []string{""}) {
		t.Error("expected an empty extra pattern to never match")
	}
}
