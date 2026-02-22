package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "" {
		t.Fatalf("expected empty region, got %q", cfg.Region)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	content := `region: us-west-2
stale_days: 30
format: json
timeout: 5m
exclude_buckets:
  - temp-bucket
  - test-bucket
exclude_prefixes:
  - logs/
`
	if err := os.WriteFile(filepath.Join(dir, ".s3spectre.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "us-west-2" {
		t.Fatalf("expected region us-west-2, got %q", cfg.Region)
	}
	if cfg.StaleDays != 30 {
		t.Fatalf("expected stale_days 30, got %d", cfg.StaleDays)
	}
	if cfg.Format != "json" {
		t.Fatalf("expected format json, got %q", cfg.Format)
	}
	if len(cfg.ExcludeBuckets) != 2 {
		t.Fatalf("expected 2 exclude_buckets, got %d", len(cfg.ExcludeBuckets))
	}
	if len(cfg.ExcludePrefixes) != 1 {
		t.Fatalf("expected 1 exclude_prefix, got %d", len(cfg.ExcludePrefixes))
	}
}

func TestLoad_YMLExtension(t *testing.T) {
	dir := t.TempDir()
	content := `region: eu-west-1`
	if err := os.WriteFile(filepath.Join(dir, ".s3spectre.yml"), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "eu-west-1" {
		t.Fatalf("expected region eu-west-1, got %q", cfg.Region)
	}
}

func TestLoad_YAMLTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".s3spectre.yaml"), []byte("region: first"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".s3spectre.yml"), []byte("region: second"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Region != "first" {
		t.Fatalf("expected .yaml to take precedence, got %q", cfg.Region)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".s3spectre.yaml"), []byte(":::invalid"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestTimeoutDuration(t *testing.T) {
	cfg := Config{Timeout: "5m"}
	if cfg.TimeoutDuration() != 5*time.Minute {
		t.Fatalf("expected 5m, got %v", cfg.TimeoutDuration())
	}

	cfg.Timeout = ""
	if cfg.TimeoutDuration() != 0 {
		t.Fatalf("expected 0 for empty, got %v", cfg.TimeoutDuration())
	}

	cfg.Timeout = "invalid"
	if cfg.TimeoutDuration() != 0 {
		t.Fatalf("expected 0 for invalid, got %v", cfg.TimeoutDuration())
	}
}
