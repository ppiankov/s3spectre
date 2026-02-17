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

func TestScanJSON_HTTPAndS3URL(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "config.json")

	content := `{"backup":"s3://json-bucket/path/file","url":"https://http-bucket.s3.us-west-2.amazonaws.com/key"}`
	if err := os.WriteFile(jsonFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanJSON(jsonFile)
	if err != nil {
		t.Fatalf("scanJSON failed: %v", err)
	}
	if len(refs) < 2 {
		t.Fatalf("Expected at least 2 references, got %d", len(refs))
	}

	buckets := make(map[string]Reference)
	for _, ref := range refs {
		buckets[ref.Bucket] = ref
	}

	ref, ok := buckets["json-bucket"]
	if !ok {
		t.Fatalf("Expected to find json-bucket")
	}
	if ref.Prefix != "path/file" {
		t.Fatalf("Expected prefix path/file, got %q", ref.Prefix)
	}

	ref, ok = buckets["http-bucket"]
	if !ok {
		t.Fatalf("Expected to find http-bucket")
	}
	if ref.Prefix != "key" {
		t.Fatalf("Expected prefix key, got %q", ref.Prefix)
	}
}

func TestScanEnv_PatternsAndComments(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "service.env")

	content := `
# S3_BUCKET=comment-bucket
S3_BUCKET=env-bucket
BUCKET_NAME="name-bucket"
AWS_BUCKET='aws-bucket'
BUCKET=plain-bucket
PATH=s3://url-bucket/prefix
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanEnv(envFile)
	if err != nil {
		t.Fatalf("scanEnv failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}

	if buckets["comment-bucket"] {
		t.Fatalf("Did not expect comment-bucket to be scanned")
	}

	expected := []string{"env-bucket", "name-bucket", "aws-bucket", "plain-bucket", "url-bucket"}
	for _, bucket := range expected {
		if !buckets[bucket] {
			t.Fatalf("Expected to find bucket %s", bucket)
		}
	}
}

func TestScanCode_DeduplicatesBucketName(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "main.go")

	content := `const url = "s3://dup-bucket/path"; const bucket = "dup-bucket"`
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}

	if len(refs) != 1 {
		t.Fatalf("Expected 1 reference, got %d", len(refs))
	}
	if refs[0].Bucket != "dup-bucket" {
		t.Fatalf("Expected bucket dup-bucket, got %q", refs[0].Bucket)
	}
	if refs[0].Prefix != "path" {
		t.Fatalf("Expected prefix path, got %q", refs[0].Prefix)
	}
}

func TestScanTerraform_ObjectResource(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "object.tf")

	content := `
resource "aws_s3_bucket_object" "object" {
  bucket = "object-bucket"
  key = "file.txt"
}
`
	if err := os.WriteFile(tfFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanTerraform(tfFile)
	if err != nil {
		t.Fatalf("scanTerraform failed: %v", err)
	}

	if len(refs) != 1 {
		t.Fatalf("Expected 1 bucket reference, got %d", len(refs))
	}
	if refs[0].Bucket != "object-bucket" {
		t.Fatalf("Expected bucket object-bucket, got %q", refs[0].Bucket)
	}
}

func TestScanYAML_CloudFormationS3Bucket(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "template.yaml")

	content := `
Resources:
  MyFunction:
    Type: AWS::Lambda::Function
    Properties:
      Code:
        S3Bucket: cf-bucket
        S3Key: code.zip
`
	if err := os.WriteFile(yamlFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanYAML(yamlFile)
	if err != nil {
		t.Fatalf("scanYAML failed: %v", err)
	}

	found := false
	for _, ref := range refs {
		if ref.Bucket == "cf-bucket" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected to find cf-bucket")
	}
}

func TestScanFile_RoutesByExtension(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "service.env")
	if err := os.WriteFile(envFile, []byte("S3_BUCKET=env-file-bucket\n"), 0644); err != nil {
		t.Fatalf("Failed to create env file: %v", err)
	}

	scanner := NewRepoScanner(tmpDir)
	refs, err := scanner.scanFile(envFile)
	if err != nil {
		t.Fatalf("scanFile failed: %v", err)
	}
	if len(refs) != 1 || refs[0].Bucket != "env-file-bucket" {
		t.Fatalf("Expected env-file-bucket reference, got %v", refs)
	}

	unknownFile := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(unknownFile, []byte("nothing here"), 0644); err != nil {
		t.Fatalf("Failed to create txt file: %v", err)
	}

	refs, err = scanner.scanFile(unknownFile)
	if err != nil {
		t.Fatalf("scanFile failed: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("Expected no references for unknown extension, got %d", len(refs))
	}
}
