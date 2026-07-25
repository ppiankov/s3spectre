package commands

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ppiankov/s3spectre/internal/s3"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

func TestEnhanceError(t *testing.T) {
	if enhanceError("op", nil, 1) != nil {
		t.Fatalf("expected nil error when input is nil")
	}

	cases := []struct {
		err      error
		contains string
	}{
		{errors.New("NoCredentialProviders"), "No AWS credentials found"},
		{errors.New("AccessDenied"), "Access Denied"},
		{errors.New("RequestLimitExceeded"), "rate limit exceeded"},
		{errors.New("no such file or directory"), "Repository path not found"},
		{errors.New("some other error"), "op failed"},
	}

	for _, tt := range cases {
		err := enhanceError("op", tt.err, 5)
		if err == nil {
			t.Fatalf("expected error for %v", tt.err)
		}
		if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.contains)) {
			t.Fatalf("expected error to contain %q, got %q", tt.contains, err.Error())
		}
	}
}

func TestPrintStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(oldLogger)
	})

	printStatus("hello %s", "world")

	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("expected output to contain message, got %q", buf.String())
	}
}

func TestGetVersion(t *testing.T) {
	version = "1.2.3"
	t.Cleanup(func() { version = "" })
	if GetVersion() != "1.2.3" {
		t.Fatalf("expected version %q, got %q", "1.2.3", GetVersion())
	}
}

func TestIsExcludedBucket(t *testing.T) {
	excludeBuckets := []string{"legacy-bucket"}
	excludePrefixes := []string{"tmp-"}

	cases := []struct {
		name string
		want bool
	}{
		{"legacy-bucket", true},
		{"tmp-scratch", true},
		{"tmp-", true},
		{"production-data", false},
		{"legacy-bucket-2", false},
	}

	for _, tt := range cases {
		if got := isExcludedBucket(tt.name, excludeBuckets, excludePrefixes); got != tt.want {
			t.Errorf("isExcludedBucket(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFilterExcludedReferences(t *testing.T) {
	refs := []scanner.Reference{
		{Bucket: "keep-me"},
		{Bucket: "legacy-bucket"},
		{Bucket: "tmp-scratch"},
	}

	filtered := filterExcludedReferences(refs, []string{"legacy-bucket"}, []string{"tmp-"})
	if len(filtered) != 1 || filtered[0].Bucket != "keep-me" {
		t.Fatalf("expected only keep-me to survive, got %+v", filtered)
	}

	unfiltered := filterExcludedReferences(refs, nil, nil)
	if len(unfiltered) != len(refs) {
		t.Fatalf("expected no filtering with empty exclude lists, got %d/%d", len(unfiltered), len(refs))
	}
}

func TestFilterExcludedBuckets(t *testing.T) {
	buckets := map[string]*s3.BucketInfo{
		"keep-me":       {Name: "keep-me"},
		"legacy-bucket": {Name: "legacy-bucket"},
		"tmp-scratch":   {Name: "tmp-scratch"},
	}

	filtered := filterExcludedBuckets(buckets, []string{"legacy-bucket"}, []string{"tmp-"})
	if len(filtered) != 1 {
		t.Fatalf("expected only keep-me to survive, got %+v", filtered)
	}
	if _, ok := filtered["keep-me"]; !ok {
		t.Fatalf("expected keep-me to survive, got %+v", filtered)
	}

	unfiltered := filterExcludedBuckets(buckets, nil, nil)
	if len(unfiltered) != len(buckets) {
		t.Fatalf("expected no filtering with empty exclude lists, got %d/%d", len(unfiltered), len(buckets))
	}
}
