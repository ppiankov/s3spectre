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

// TestRepoScanner_ExcludesVendorDirectory is the core WO-52 regression:
// third-party dependency code under vendor/ must never be scanned, even
// though it has a normal, non-hidden directory name and recognized file
// extensions. Reproduced against a real vendored Go backend service where
// 100% of findings came from vendor/github.com/aws/aws-sdk-go source.
func TestRepoScanner_ExcludesVendorDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	vendorDir := filepath.Join(tmpDir, "vendor", "pkg")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "file.go"), []byte(`bucket = "vendored-bucket"`), 0644); err != nil {
		t.Fatalf("Failed to create vendored test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`bucket = "app-bucket"`), 0644); err != nil {
		t.Fatalf("Failed to create app test file: %v", err)
	}

	scanner := NewRepoScanner(tmpDir)
	refs, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}
	if buckets["vendored-bucket"] {
		t.Fatalf("expected a bucket reference inside vendor/ to be excluded, got refs: %+v", refs)
	}
	if !buckets["app-bucket"] {
		t.Fatalf("expected the app's own bucket reference to still be captured, got refs: %+v", refs)
	}
}

// TestRepoScanner_ExcludesOtherDependencyDirectories covers the rest of the
// WO-52 denylist as a table, one directory at a time.
func TestRepoScanner_ExcludesOtherDependencyDirectories(t *testing.T) {
	dirNames := []string{"node_modules", "target", "dist", "build", "Pods", "site-packages", "bower_components"}

	for _, dirName := range dirNames {
		t.Run(dirName, func(t *testing.T) {
			tmpDir := t.TempDir()

			excludedDir := filepath.Join(tmpDir, dirName)
			if err := os.MkdirAll(excludedDir, 0755); err != nil {
				t.Fatalf("Failed to create %s dir: %v", dirName, err)
			}
			if err := os.WriteFile(filepath.Join(excludedDir, "file.py"), []byte(`bucket = "excluded-bucket"`), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			scanner := NewRepoScanner(tmpDir)
			refs, err := scanner.Scan(context.Background())
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			for _, ref := range refs {
				if ref.Bucket == "excluded-bucket" {
					t.Fatalf("expected a bucket reference inside %s/ to be excluded, got refs: %+v", dirName, refs)
				}
			}
		})
	}
}

// TestRepoScanner_DoesNotExcludeSimilarlyNamedDirectory guards the
// exact-basename-match requirement: a directory whose name merely contains
// "vendor" as a substring, rather than being named exactly "vendor", must
// still be scanned.
func TestRepoScanner_DoesNotExcludeSimilarlyNamedDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	dir := filepath.Join(tmpDir, "vendor-scripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.py"), []byte(`bucket = "vendor-scripts-bucket"`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	scanner := NewRepoScanner(tmpDir)
	refs, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	found := false
	for _, ref := range refs {
		if ref.Bucket == "vendor-scripts-bucket" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a directory named 'vendor-scripts' (substring match only) not to be excluded, got refs: %+v", refs)
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

// TestScanJSON_BucketNamePattern_QuoteRequiredAndPlaceholderFiltered mirrors
// the WO-49 scanCode fix for scanJSON's parallel use of bucketNamePattern:
// only a quoted string value counts as a bucket reference, and a generic
// placeholder token is never reported as real.
func TestScanJSON_BucketNamePattern_QuoteRequiredAndPlaceholderFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "config.json")

	// Note: bucketNamePattern's trigger keyword must be immediately followed
	// by [\s:=]+ then a quoted value -- a standard JSON quoted key
	// (`"bucket":`) has its own closing quote in between, so it never
	// matches this pattern (a pre-existing scanJSON limitation, unrelated to
	// this fix). Using the unquoted-key form the regex actually supports.
	content := "bucket: \"acme-real-bucket\"\n// example: bucket: \"my-bucket\"\n"
	if err := os.WriteFile(jsonFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanJSON(jsonFile)
	if err != nil {
		t.Fatalf("scanJSON failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}
	if !buckets["acme-real-bucket"] {
		t.Fatalf("expected a real quoted bucket name to be captured, got refs: %+v", refs)
	}
	if buckets["my-bucket"] {
		t.Fatalf("expected the placeholder 'my-bucket' to be filtered out, got refs: %+v", refs)
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

// TestScanCode_UnquotedAttributeAccessNotCapturedAsBucket is the core WO-49
// regression: bucketNamePattern previously matched any token after "bucket ="
// regardless of quoting, so a variable holding a parsed value (an unquoted
// attribute-access expression) was captured as if it were a literal bucket
// name. Confirmed against two independent real codebases before this fix.
func TestScanCode_UnquotedAttributeAccessNotCapturedAsBucket(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "main.py")

	content := "bucket = parsed.netloc\nif not bucket:\n    raise ValueError()\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}
	for _, ref := range refs {
		if ref.Bucket == "parsed.netloc" {
			t.Fatalf("expected unquoted attribute-access expression not to be captured as a bucket name, got refs: %+v", refs)
		}
	}
}

// TestScanCode_UnquotedSelfAttributeNotCapturedAsBucket guards the second
// real-world shape of the same bug: a keyword-argument-style assignment
// (`Bucket=self.bucket_name`) where the value is an unquoted self-attribute
// reference, not a string literal.
func TestScanCode_UnquotedSelfAttributeNotCapturedAsBucket(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "client.py")

	content := "s3.upload_file(Filename=path, Bucket=self.bucket_name, Key=key)\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}
	for _, ref := range refs {
		if ref.Bucket == "self.bucket_name" {
			t.Fatalf("expected unquoted self-attribute reference not to be captured as a bucket name, got refs: %+v", refs)
		}
	}
}

// TestScanCode_QuotedBucketNameStillCaptured is the regression guard: the
// WO-49 fix must not break the common, legitimate case of a bucket name
// assigned as an actual string literal, in either quote style.
func TestScanCode_QuotedBucketNameStillCaptured(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "config.py")

	content := "bucket = \"my-real-bucket\"\nother_bucket = 'my-other-bucket'\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}
	if !buckets["my-real-bucket"] {
		t.Fatalf("expected double-quoted literal bucket name to still be captured, got refs: %+v", refs)
	}
	if !buckets["my-other-bucket"] {
		t.Fatalf("expected single-quoted literal bucket name to still be captured, got refs: %+v", refs)
	}
}

// TestScanCode_DocstringPlaceholderNotCapturedAsBucket guards the second
// WO-49 fix: a generic illustrative example like "s3://bucket/key" inside a
// docstring or comment must not be reported as a real bucket reference,
// since real S3 bucket names are globally unique and no production bucket
// is actually named the bare word "bucket".
func TestScanCode_DocstringPlaceholderNotCapturedAsBucket(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "client.py")

	content := "    \"\"\"Download via aioboto3 from s3://bucket/key.\"\"\"\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected a generic 's3://bucket/key' docstring placeholder to produce no references, got %+v", refs)
	}
}

// TestScanCode_RealBucketNameSharingPlaceholderWordStillCaptured guards
// against the placeholder denylist over-firing: a real bucket name must
// still be captured even if part of it resembles a denylisted word, and a
// genuinely distinct real bucket name is never suppressed.
func TestScanCode_RealBucketNameSharingPlaceholderWordStillCaptured(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "client.py")

	content := "url = \"s3://acme-prod-data-bucket-42/key\"\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}
	if len(refs) != 1 || refs[0].Bucket != "acme-prod-data-bucket-42" {
		t.Fatalf("expected a real, non-placeholder bucket name to still be captured, got %+v", refs)
	}
}

// TestScanEnv_S3URLPlaceholderFiltered is the WO-50 extension of the WO-49
// placeholder-suppression fix to scanEnv's own s3URLPattern usage: a generic
// example URL must not produce a phantom reference, while a real bucket name
// elsewhere in the same file is still captured. The placeholder line is
// deliberately NOT a comment (scanEnv already skips full-line comments before
// reaching s3URLPattern) so this test actually exercises the new filter
// rather than passing vacuously via the pre-existing comment-skip.
func TestScanEnv_S3URLPlaceholderFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "service.env")

	content := "EXAMPLE_S3_PATH=s3://bucket/key\nS3_URL=s3://acme-real-bucket/data\n"
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
	if buckets["bucket"] {
		t.Fatalf("expected the placeholder 'bucket' to be filtered out, got refs: %+v", refs)
	}
	if !buckets["acme-real-bucket"] {
		t.Fatalf("expected a real bucket name to still be captured, got refs: %+v", refs)
	}
}

// TestScanYAML_S3URLPlaceholderFiltered mirrors the same fix for scanYAML's
// s3URLPattern usage (yamlBucketPattern's own quote-optionality is untouched
// by WO-50, only the shared URL pattern is filtered).
func TestScanYAML_S3URLPlaceholderFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "template.yaml")

	content := "# example url: s3://bucket/key\nstorage:\n  url: s3://acme-real-bucket/prefix\n"
	if err := os.WriteFile(yamlFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanYAML(yamlFile)
	if err != nil {
		t.Fatalf("scanYAML failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}
	if buckets["bucket"] {
		t.Fatalf("expected the placeholder 'bucket' to be filtered out, got refs: %+v", refs)
	}
	if !buckets["acme-real-bucket"] {
		t.Fatalf("expected a real bucket name to still be captured, got refs: %+v", refs)
	}
}

// TestScanTerraform_S3URLPlaceholderFiltered mirrors the same fix for
// scanTerraform's s3URLPattern usage (tfBucketNameAttr, the resource-block
// bucket attribute extractor, is untouched by WO-50).
func TestScanTerraform_S3URLPlaceholderFiltered(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "main.tf")

	content := "# example: s3://bucket/key\nlocals {\n  backup_url = \"s3://acme-real-bucket/backup\"\n}\n"
	if err := os.WriteFile(tfFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanTerraform(tfFile)
	if err != nil {
		t.Fatalf("scanTerraform failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}
	if buckets["bucket"] {
		t.Fatalf("expected the placeholder 'bucket' to be filtered out, got refs: %+v", refs)
	}
	if !buckets["acme-real-bucket"] {
		t.Fatalf("expected a real bucket name to still be captured, got refs: %+v", refs)
	}
}

// TestScanTerraform_UnresolvedInterpolationNotCapturedAsBucket is the core
// WO-51 regression: tfBucketNameAttr previously captured a Terraform
// interpolation expression verbatim as if it were a literal bucket name.
// Reproduced against a real, unrelated 2,938-file Terraform codebase where
// this accounted for 85% of MISSING_BUCKET findings in a single scan.
func TestScanTerraform_UnresolvedInterpolationNotCapturedAsBucket(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "s3.tf")

	content := `
resource "aws_s3_bucket" "landing" {
  bucket = "${local.cluster_name}-landing"
}
`
	if err := os.WriteFile(tfFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanTerraform(tfFile)
	if err != nil {
		t.Fatalf("scanTerraform failed: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected an unresolved interpolation expression not to be captured as a bucket name, got refs: %+v", refs)
	}
}

// TestScanTerraform_PlainLiteralStillCaptured is the regression guard: the
// WO-51 fix must not affect the common, legitimate case of a bucket name
// assigned as an actual literal string.
func TestScanTerraform_PlainLiteralStillCaptured(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "s3.tf")

	content := `
resource "aws_s3_bucket" "backups" {
  bucket = "my-real-bucket"
}
`
	if err := os.WriteFile(tfFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanTerraform(tfFile)
	if err != nil {
		t.Fatalf("scanTerraform failed: %v", err)
	}
	if len(refs) != 1 || refs[0].Bucket != "my-real-bucket" {
		t.Fatalf("expected the plain literal bucket name to still be captured, got refs: %+v", refs)
	}
}

// TestScanTerraform_MixedInterpolatedAndLiteralBuckets covers a file with
// one interpolated resource (suppressed) and one plain-literal resource
// (still captured) -- the shape actually seen in the real codebase this WO
// was reproduced against.
func TestScanTerraform_MixedInterpolatedAndLiteralBuckets(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "s3.tf")

	content := `
resource "aws_s3_bucket" "loki" {
  bucket = "${local.cluster_name}-loki"
}

resource "aws_s3_bucket" "vault" {
  bucket = "acme-vault-prod"
}
`
	if err := os.WriteFile(tfFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanTerraform(tfFile)
	if err != nil {
		t.Fatalf("scanTerraform failed: %v", err)
	}
	if len(refs) != 1 || refs[0].Bucket != "acme-vault-prod" {
		t.Fatalf("expected only the plain literal bucket to be captured, got refs: %+v", refs)
	}
}

func TestHasUnresolvedInterpolation(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"single local reference", "${local.cluster_name}-landing", true},
		{"multiple interpolations", "${local.common_tags.Project}-${local.common_tags.Environment}-web-public", true},
		{"plain literal", "my-real-bucket", false},
		{"empty", "", false},
	}
	for _, tt := range cases {
		if got := hasUnresolvedInterpolation(tt.value); got != tt.want {
			t.Errorf("hasUnresolvedInterpolation(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

// TestScanCode_SDKConstantSuffixNotCapturedAsBucket is the core WO-53
// regression: bucketNamePattern previously had no boundary before the
// keyword alternation, so it matched "Bucket" embedded as a bare camelCase
// suffix inside an unrelated identifier. Reproduced against a real vendored
// AWS SDK source file where API operation-name and error-code constants
// (opCreateBucket, ErrCodeNoSuchBucket) were captured as if they were
// literal bucket names.
func TestScanCode_SDKConstantSuffixNotCapturedAsBucket(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "api.go")

	content := "const opCreateBucket = \"CreateBucket\"\n//   - ErrCodeNoSuchBucket \"NoSuchBucket\"\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}
	for _, ref := range refs {
		if ref.Bucket == "CreateBucket" || ref.Bucket == "NoSuchBucket" {
			t.Fatalf("expected an SDK constant with 'Bucket' as a bare suffix not to be captured, got refs: %+v", refs)
		}
	}
}

// TestScanCode_SnakeCaseAndS3PrefixedBucketNamesStillCaptured is the
// regression guard: the WO-53 fix must not reject standalone keyword usage,
// nor snake_case/kebab-case identifiers where "bucket" is a genuine
// standalone word component (separated by "_" or "-", not concatenated
// like the SDK-constant case above).
func TestScanCode_SnakeCaseAndS3PrefixedBucketNamesStillCaptured(t *testing.T) {
	tmpDir := t.TempDir()
	codeFile := filepath.Join(tmpDir, "config.py")

	content := "bucket = \"acme-standalone-bucket\"\n" +
		"other_bucket = 'acme-snake-case-bucket'\n" +
		"s3_bucket = \"acme-s3-prefixed-bucket\"\n" +
		"s3Bucket = \"acme-camelcase-s3-bucket\"\n"
	if err := os.WriteFile(codeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	refs, err := scanCode(codeFile)
	if err != nil {
		t.Fatalf("scanCode failed: %v", err)
	}

	buckets := make(map[string]bool)
	for _, ref := range refs {
		buckets[ref.Bucket] = true
	}
	for _, want := range []string{"acme-standalone-bucket", "acme-snake-case-bucket", "acme-s3-prefixed-bucket", "acme-camelcase-s3-bucket"} {
		if !buckets[want] {
			t.Fatalf("expected %q to still be captured, got refs: %+v", want, refs)
		}
	}
}

func TestIsPlaceholderBucketName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"bucket", true},
		{"BUCKET", true},
		{"my-bucket", true},
		{"example-bucket", true},
		{"acme-prod-data-bucket-42", false},
		{"", false},
	}
	for _, tt := range cases {
		if got := isPlaceholderBucketName(tt.name); got != tt.want {
			t.Errorf("isPlaceholderBucketName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
