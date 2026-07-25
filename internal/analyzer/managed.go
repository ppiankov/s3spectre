package analyzer

import "strings"

// managedBucketPrefixes lists known AWS-managed/service-created bucket naming
// conventions. Buckets matching one of these look "unused" by generic
// heuristics (empty, no code references, no recent activity) but are created
// and required by another AWS service -- deleting them breaks that service.
var managedBucketPrefixes = []string{
	"aws-cloudtrail-logs-",
	"aws-config-bucket-",
	"elasticloadbalancing-",
	"cf-templates-",
}

// IsServiceManagedBucket reports whether name matches a known AWS-managed
// bucket naming convention (CloudTrail, AWS Config, ELB/ALB access logs,
// CloudFormation templates, etc).
func IsServiceManagedBucket(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range managedBucketPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// managedBucketMessage is the shared non-actionable message used by both scan
// and discover when a bucket matches a known managed naming pattern.
const managedBucketMessage = "Bucket matches an AWS-managed naming pattern (e.g. CloudTrail/Config/ELB logs); service-managed, review before deleting"
