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
