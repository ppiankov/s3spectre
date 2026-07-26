package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ppiankov/s3spectre/internal/report"
)

func TestDiscoverFlagDefaults(t *testing.T) {
	if discoverFlags.allRegions != true {
		t.Fatalf("expected all-regions default true, got %v", discoverFlags.allRegions)
	}
	if discoverFlags.outputFormat != "text" {
		t.Fatalf("expected default format 'text', got %q", discoverFlags.outputFormat)
	}
	if discoverFlags.maxConcurrency != 10 {
		t.Fatalf("expected default concurrency 10, got %d", discoverFlags.maxConcurrency)
	}
	if discoverFlags.ageThresholdDays != 365 {
		t.Fatalf("expected default age-threshold-days 365, got %d", discoverFlags.ageThresholdDays)
	}
	if discoverFlags.inactiveDays != 180 {
		t.Fatalf("expected default inactive-days 180, got %d", discoverFlags.inactiveDays)
	}
	if discoverFlags.riskThreshold != 100 {
		t.Fatalf("expected default risk-threshold 100, got %d", discoverFlags.riskThreshold)
	}
	if discoverCmd.Flags().Lookup("format").DefValue != "text" {
		t.Fatalf("expected flag default format text, got %q", discoverCmd.Flags().Lookup("format").DefValue)
	}
	if discoverCmd.Flags().Lookup("estimate-cost").DefValue != "false" {
		t.Fatalf("expected flag default estimate-cost false, got %q", discoverCmd.Flags().Lookup("estimate-cost").DefValue)
	}
	if discoverCmd.Flags().Lookup("suggest-lifecycle-policy").DefValue != "false" {
		t.Fatalf("expected flag default suggest-lifecycle-policy false, got %q", discoverCmd.Flags().Lookup("suggest-lifecycle-policy").DefValue)
	}
	if discoverCmd.Flags().Lookup("group-by-tag").DefValue != "" {
		t.Fatalf("expected flag default group-by-tag empty, got %q", discoverCmd.Flags().Lookup("group-by-tag").DefValue)
	}
}

// TestRunDiscover_ConfigWiring verifies the analyzer.DiscoveryConfig built in
// runDiscover carries the new flags/config fields through correctly. Kept as
// a direct construction test (mirroring the shape runDiscover builds) since
// runDiscover itself requires a live AWS client and isn't unit-testable.
func TestRunDiscover_ConfigWiring(t *testing.T) {
	origCfg := cfg
	origSuggest := discoverFlags.suggestLifecycle
	origEstimate := discoverFlags.estimateCost
	origGroupByTag := discoverFlags.groupByTag
	t.Cleanup(func() {
		cfg = origCfg
		discoverFlags.suggestLifecycle = origSuggest
		discoverFlags.estimateCost = origEstimate
		discoverFlags.groupByTag = origGroupByTag
	})

	cfg.PublicBucketAllowlistPatterns = []string{"my-custom-pattern"}
	discoverFlags.suggestLifecycle = true
	discoverFlags.estimateCost = true
	discoverFlags.groupByTag = "Team"

	config := buildDiscoveryConfig()
	if len(config.PublicBucketAllowlistPatterns) != 1 || config.PublicBucketAllowlistPatterns[0] != "my-custom-pattern" {
		t.Fatalf("expected config allowlist patterns to wire through, got %v", config.PublicBucketAllowlistPatterns)
	}
	if !config.SuggestLifecyclePolicy {
		t.Fatal("expected SuggestLifecyclePolicy to wire through from the flag")
	}
	if !config.EstimateCost {
		t.Fatal("expected EstimateCost to wire through from the flag")
	}
	if config.GroupByTag != "Team" {
		t.Fatalf("expected GroupByTag to wire through from the flag, got %q", config.GroupByTag)
	}
}

func TestApplyConfigToDiscoverFlags_RiskThreshold(t *testing.T) {
	origThreshold := discoverFlags.riskThreshold
	origCfg := cfg
	t.Cleanup(func() {
		discoverFlags.riskThreshold = origThreshold
		cfg = origCfg
		_ = discoverCmd.Flags().Set("risk-threshold", "100")
		discoverCmd.Flags().Lookup("risk-threshold").Changed = false
	})

	cfg.RiskThreshold = 40
	discoverCmd.Flags().Lookup("risk-threshold").Changed = false
	applyConfigToDiscoverFlags(discoverCmd)
	if discoverFlags.riskThreshold != 40 {
		t.Fatalf("expected config risk_threshold 40 to apply when flag not set, got %d", discoverFlags.riskThreshold)
	}

	discoverFlags.riskThreshold = 75
	discoverCmd.Flags().Lookup("risk-threshold").Changed = true
	applyConfigToDiscoverFlags(discoverCmd)
	if discoverFlags.riskThreshold != 75 {
		t.Fatalf("expected explicit --risk-threshold to win over config, got %d", discoverFlags.riskThreshold)
	}
}

func TestDiscoverSelectReporter(t *testing.T) {
	var buf bytes.Buffer

	reporter, err := selectReporter("json", &buf)
	if err != nil {
		t.Fatalf("expected no error for json, got %v", err)
	}
	if _, ok := reporter.(*report.JSONReporter); !ok {
		t.Fatalf("expected JSONReporter, got %T", reporter)
	}

	reporter, err = selectReporter("text", &buf)
	if err != nil {
		t.Fatalf("expected no error for text, got %v", err)
	}
	if _, ok := reporter.(*report.TextReporter); !ok {
		t.Fatalf("expected TextReporter, got %T", reporter)
	}

	reporter, err = selectReporter("sarif", &buf)
	if err != nil {
		t.Fatalf("expected no error for sarif, got %v", err)
	}
	if _, ok := reporter.(*report.SARIFReporter); !ok {
		t.Fatalf("expected SARIFReporter, got %T", reporter)
	}

	reporter, err = selectReporter("markdown", &buf)
	if err != nil {
		t.Fatalf("expected no error for markdown, got %v", err)
	}
	if _, ok := reporter.(*report.MarkdownReporter); !ok {
		t.Fatalf("expected MarkdownReporter, got %T", reporter)
	}

	_, err = selectReporter("yaml", &buf)
	if err == nil {
		t.Fatalf("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
