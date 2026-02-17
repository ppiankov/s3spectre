package s3

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

func TestNewInspector_DefaultConcurrency(t *testing.T) {
	client := &Client{config: aws.Config{Region: "us-east-1"}}
	inspector := NewInspector(client, 0)
	if inspector.concurrency != 10 {
		t.Fatalf("expected default concurrency 10, got %d", inspector.concurrency)
	}
}

func TestInspector_SetRegionsAndAllRegions(t *testing.T) {
	client := &Client{config: aws.Config{Region: "us-east-1"}}
	inspector := NewInspector(client, 5)

	inspector.SetRegions([]string{"us-west-2"})
	if inspector.allRegions {
		t.Fatalf("expected allRegions false after SetRegions")
	}
	if len(inspector.regions) != 1 || inspector.regions[0] != "us-west-2" {
		t.Fatalf("unexpected regions: %v", inspector.regions)
	}

	inspector.SetAllRegions(true)
	if !inspector.allRegions {
		t.Fatalf("expected allRegions true after SetAllRegions")
	}
}

func TestInspector_DetermineRegions(t *testing.T) {
	client := &Client{config: aws.Config{Region: "us-east-1"}}
	inspector := NewInspector(client, 5)

	inspector.SetRegions([]string{"us-west-2"})
	regions, err := inspector.determineRegions(context.Background())
	if err != nil {
		t.Fatalf("determineRegions failed: %v", err)
	}
	if len(regions) != 1 || regions[0] != "us-west-2" {
		t.Fatalf("unexpected regions: %v", regions)
	}

	inspector.regions = nil
	inspector.allRegions = false
	regions, err = inspector.determineRegions(context.Background())
	if err != nil {
		t.Fatalf("determineRegions failed: %v", err)
	}
	if len(regions) != 1 || regions[0] != "us-east-1" {
		t.Fatalf("unexpected regions: %v", regions)
	}
}

func TestInspector_ReportProgress(t *testing.T) {
	client := &Client{config: aws.Config{Region: "us-east-1"}}
	inspector := NewInspector(client, 5)

	var gotCurrent, gotTotal int
	var gotMessage string
	inspector.SetProgressCallback(func(current, total int, message string) {
		gotCurrent = current
		gotTotal = total
		gotMessage = message
	})

	inspector.reportProgress(2, 3, "working")
	if gotCurrent != 2 || gotTotal != 3 || gotMessage != "working" {
		t.Fatalf("unexpected progress values: %d %d %s", gotCurrent, gotTotal, gotMessage)
	}
}

func TestInspector_ExtractPrefixes(t *testing.T) {
	client := &Client{config: aws.Config{Region: "us-east-1"}}
	inspector := NewInspector(client, 5)

	refs := []scanner.Reference{
		{Bucket: "a", Prefix: "logs"},
		{Bucket: "a", Prefix: ""},
		{Bucket: "b", Prefix: "logs"},
		{Bucket: "c", Prefix: "data"},
	}

	prefixes := inspector.extractPrefixes(refs)
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 unique prefixes, got %d", len(prefixes))
	}
	found := map[string]bool{}
	for _, prefix := range prefixes {
		found[prefix] = true
	}
	if !found["logs"] || !found["data"] {
		t.Fatalf("unexpected prefixes: %v", prefixes)
	}
}

func TestFormatError(t *testing.T) {
	if formatError("op", "bucket", nil) != "" {
		t.Fatalf("expected empty error string for nil error")
	}

	accessErr := formatError("op", "bucket", errors.New("AccessDenied"))
	if !strings.Contains(accessErr, "Access Denied") {
		t.Fatalf("unexpected access error: %s", accessErr)
	}

	missingErr := formatError("op", "bucket", errors.New("NoSuchBucket"))
	if !strings.Contains(missingErr, "Bucket does not exist") {
		t.Fatalf("unexpected missing error: %s", missingErr)
	}

	rateErr := formatError("op", "bucket", errors.New("SlowDown"))
	if !strings.Contains(rateErr, "Rate limit exceeded") {
		t.Fatalf("unexpected rate error: %s", rateErr)
	}

	genericErr := formatError("op", "bucket", errors.New("boom"))
	if !strings.Contains(genericErr, "boom") {
		t.Fatalf("unexpected generic error: %s", genericErr)
	}
}
