package remediation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSuggestLifecyclePolicy_ProducesValidJSON(t *testing.T) {
	s := SuggestLifecyclePolicy("my-bucket")

	var parsed map[string]any
	if err := json.Unmarshal([]byte(s.JSON), &parsed); err != nil {
		t.Fatalf("expected valid JSON snippet, got parse error: %v\nsnippet:\n%s", err, s.JSON)
	}
	rules, ok := parsed["Rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("expected exactly one rule in Rules, got: %v", parsed["Rules"])
	}
}

func TestSuggestLifecyclePolicy_TerraformContainsBucketName(t *testing.T) {
	s := SuggestLifecyclePolicy("my-bucket")

	if !strings.Contains(s.Terraform, `bucket = "my-bucket"`) {
		t.Fatalf("expected Terraform snippet to reference the bucket name, got:\n%s", s.Terraform)
	}
	if !strings.Contains(s.Terraform, "aws_s3_bucket_lifecycle_configuration") {
		t.Fatalf("expected Terraform snippet to use the lifecycle-configuration resource, got:\n%s", s.Terraform)
	}
	if !strings.Contains(s.Terraform, "noncurrent_version_expiration") {
		t.Fatalf("expected Terraform snippet to configure noncurrent-version expiration, got:\n%s", s.Terraform)
	}
}

func TestSuggestLifecyclePolicy_Deterministic(t *testing.T) {
	a := SuggestLifecyclePolicy("repeatable-bucket")
	b := SuggestLifecyclePolicy("repeatable-bucket")

	if a.JSON != b.JSON || a.Terraform != b.Terraform {
		t.Fatal("expected SuggestLifecyclePolicy to be deterministic for the same bucket name")
	}
}

// TestTerraformResourceName_SanitizesDots guards against a bucket name
// containing dots (a valid S3 naming character) producing an invalid
// Terraform resource identifier.
func TestTerraformResourceName_SanitizesDots(t *testing.T) {
	s := SuggestLifecyclePolicy("my.bucket.with.dots")

	if strings.Contains(s.Terraform, `_lifecycle_configuration" "my.bucket`) {
		t.Fatalf("expected dots to be sanitized out of the Terraform resource name, got:\n%s", s.Terraform)
	}
	if got := terraformResourceName("my.bucket.with.dots"); strings.Contains(got, ".") {
		t.Fatalf("expected no dots in sanitized resource name, got %q", got)
	}
}

// TestTerraformResourceName_HandlesLeadingDigit guards against a bucket name
// starting with a digit (valid for S3) producing an identifier Terraform
// would reject (resource names cannot start with a digit).
func TestTerraformResourceName_HandlesLeadingDigit(t *testing.T) {
	got := terraformResourceName("123-my-bucket")
	if len(got) == 0 || (got[0] >= '0' && got[0] <= '9') {
		t.Fatalf("expected sanitized name to not start with a digit, got %q", got)
	}
}

func TestTerraformResourceName_EmptyInput(t *testing.T) {
	if got := terraformResourceName(""); got == "" {
		t.Fatal("expected a non-empty fallback resource name for empty input")
	}
}
