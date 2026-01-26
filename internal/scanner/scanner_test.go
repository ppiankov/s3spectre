package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoScanner(t *testing.T) {
	// Create a temporary test directory
	tmpDir := t.TempDir()

	// Create test files with S3 references
	testFiles := map[string]string{
		"config.yaml": `
app:
  bucket: test-bucket-123
  prefix: s3://test-bucket-123/data/
`,
		"app.py": `
import boto3
BUCKET = "my-python-bucket"
s3_client = boto3.client('s3')
s3_client.upload_file("file.txt", "my-python-bucket", "key")
`,
		"main.tf": `
resource "aws_s3_bucket" "data" {
  bucket = "terraform-bucket"
}
`,
	}

	for filename, content := range testFiles {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create scanner and scan
	scanner := NewRepoScanner(tmpDir)
	refs, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify we found references
	if len(refs) == 0 {
		t.Fatal("Expected to find S3 references, got none")
	}

	// Check for specific buckets
	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}

	expectedBuckets := []string{"test-bucket-123", "my-python-bucket", "terraform-bucket"}
	for _, expected := range expectedBuckets {
		if !buckets[expected] {
			t.Errorf("Expected to find bucket %s, but it was not found", expected)
		}
	}
}

func TestScanYAML(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")

	content := `
storage:
  bucket: yaml-test-bucket
  url: s3://yaml-test-bucket/prefix/data
`
	if err := os.WriteFile(yamlFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanYAML(yamlFile)
	if err != nil {
		t.Fatalf("scanYAML failed: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("Expected to find references in YAML")
	}

	found := false
	for _, ref := range refs {
		if ref.Bucket == "yaml-test-bucket" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find yaml-test-bucket")
	}
}

func TestScanTerraform(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "main.tf")

	content := `
resource "aws_s3_bucket" "app_data" {
  bucket = "tf-test-bucket"
}

resource "aws_s3_bucket" "backups" {
  bucket = "tf-backup-bucket"
}
`
	if err := os.WriteFile(tfFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanTerraform(tfFile)
	if err != nil {
		t.Fatalf("scanTerraform failed: %v", err)
	}

	if len(refs) < 2 {
		t.Fatalf("Expected to find at least 2 buckets, found %d", len(refs))
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}

	if !buckets["tf-test-bucket"] {
		t.Error("Expected to find tf-test-bucket")
	}
	if !buckets["tf-backup-bucket"] {
		t.Error("Expected to find tf-backup-bucket")
	}
}

func TestDetectContext(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"s3.get_object(Bucket='test', Key='file')", "read"},
		{"s3.put_object(Bucket='test', Key='file', Body=data)", "write"},
		{"s3.upload_file('local', 'bucket', 'key')", "write"},
		{"s3.download_file('bucket', 'key', 'local')", "read"},
		{"s3.list_objects(Bucket='test')", "list"},
		{"bucket = 'my-bucket'", "unknown"},
	}

	for _, tt := range tests {
		result := detectContext(tt.line)
		if result != tt.expected {
			t.Errorf("detectContext(%q) = %q, want %q", tt.line, result, tt.expected)
		}
	}
}
