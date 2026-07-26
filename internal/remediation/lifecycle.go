// Package remediation generates deterministic, human-reviewed remediation
// snippets from data s3spectre has already collected. It never calls any AWS
// write API and never applies anything itself -- every snippet is a
// suggestion for an operator to review, copy, and apply manually.
package remediation

import (
	"fmt"
	"strings"
)

// defaultNoncurrentVersionExpirationDays is the suggested retention window
// for noncurrent object versions in a version-sprawl bucket. A conservative
// default; operators should tune it to their own retention requirements
// before applying.
const defaultNoncurrentVersionExpirationDays = 90

// LifecyclePolicySuggestion holds equivalent lifecycle-rule snippets in two
// common formats, for an operator to review and apply through their own
// deployment path (console, CLI, or Terraform).
type LifecyclePolicySuggestion struct {
	JSON      string `json:"json"`
	Terraform string `json:"terraform"`
}

// SuggestLifecyclePolicy generates a noncurrent-version-expiration lifecycle
// rule suggestion for bucketName. Purely a text-generation function over
// already-known data -- no AWS API calls, no side effects.
func SuggestLifecyclePolicy(bucketName string) LifecyclePolicySuggestion {
	jsonSnippet := fmt.Sprintf(`{
  "Rules": [
    {
      "ID": "expire-noncurrent-versions",
      "Status": "Enabled",
      "NoncurrentVersionExpiration": {
        "NoncurrentDays": %d
      }
    }
  ]
}`, defaultNoncurrentVersionExpirationDays)

	terraformSnippet := fmt.Sprintf(`resource "aws_s3_bucket_lifecycle_configuration" %q {
  bucket = %q

  rule {
    id     = "expire-noncurrent-versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = %d
    }
  }
}`, terraformResourceName(bucketName), bucketName, defaultNoncurrentVersionExpirationDays)

	return LifecyclePolicySuggestion{JSON: jsonSnippet, Terraform: terraformSnippet}
}

// terraformResourceName sanitizes an S3 bucket name into a valid Terraform
// resource local name: must start with a letter or underscore and contain
// only letters, digits, underscores, or hyphens.
func terraformResourceName(bucketName string) string {
	var b strings.Builder
	for _, r := range bucketName {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := b.String()
	if name == "" {
		return "_bucket"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "_" + name
	}
	return name
}
