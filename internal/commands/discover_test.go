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
	if discoverCmd.Flags().Lookup("format").DefValue != "text" {
		t.Fatalf("expected flag default format text, got %q", discoverCmd.Flags().Lookup("format").DefValue)
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

	_, err = selectReporter("yaml", &buf)
	if err == nil {
		t.Fatalf("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
