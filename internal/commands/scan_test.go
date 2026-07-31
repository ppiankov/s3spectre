package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ppiankov/s3spectre/internal/report"
)

func TestScanFlagDefaults(t *testing.T) {
	if scanFlags.repoPath != "." {
		t.Fatalf("expected default repo path '.', got %q", scanFlags.repoPath)
	}
	if scanFlags.allRegions != true {
		t.Fatalf("expected all-regions default true, got %v", scanFlags.allRegions)
	}
	if scanFlags.outputFormat != "text" {
		t.Fatalf("expected default format 'text', got %q", scanFlags.outputFormat)
	}
	if scanFlags.maxConcurrency != 10 {
		t.Fatalf("expected default concurrency 10, got %d", scanFlags.maxConcurrency)
	}
	if scanFlags.staleThresholdDays != 90 {
		t.Fatalf("expected default stale-days 90, got %d", scanFlags.staleThresholdDays)
	}
	if scanFlags.unusedThresholdDays != 180 {
		t.Fatalf("expected default unused-threshold-days 180, got %d", scanFlags.unusedThresholdDays)
	}
	if scanCmd.Flags().Lookup("format").DefValue != "text" {
		t.Fatalf("expected flag default format text, got %q", scanCmd.Flags().Lookup("format").DefValue)
	}
}

// TestShouldIncludeReferences is the WO-56 regression: SARIF's inline
// PR-annotation capability depends on scan's collected references reaching
// the report, but that was previously gated entirely behind
// --include-references -- a flag whose only other purpose is JSON output
// verbosity. Running `s3spectre scan --format sarif` without that flag (the
// natural, undocumented-otherwise invocation) silently produced SARIF output
// with no file/line locations, so no finding could ever get an inline
// annotation.
func TestShouldIncludeReferences(t *testing.T) {
	cases := []struct {
		name                  string
		format                string
		includeReferencesFlag bool
		want                  bool
	}{
		{"sarif format always includes references", "sarif", false, true},
		{"sarif format with flag also set", "sarif", true, true},
		{"json format respects the flag when unset", "json", false, false},
		{"json format respects the flag when set", "json", true, true},
		{"text format respects the flag when unset", "text", false, false},
		{"text format respects the flag when set", "text", true, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldIncludeReferences(tt.format, tt.includeReferencesFlag); got != tt.want {
				t.Errorf("shouldIncludeReferences(%q, %v) = %v, want %v", tt.format, tt.includeReferencesFlag, got, tt.want)
			}
		})
	}
}

func TestScanSelectReporter(t *testing.T) {
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

	_, err = selectReporter("xml", &buf)
	if err == nil {
		t.Fatalf("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
