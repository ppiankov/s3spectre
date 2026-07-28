package scanner

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var (
	// Terraform S3 resource patterns
	tfS3BucketResource = regexp.MustCompile(`resource\s+"aws_s3_bucket"\s+"[^"]+"\s+\{`)
	tfBucketNameAttr   = regexp.MustCompile(`bucket\s+=\s+"([^"]+)"`)
	tfS3ObjectResource = regexp.MustCompile(`resource\s+"aws_s3_(?:bucket_)?object"\s+"[^"]+"\s+\{`)
)

// hasUnresolvedInterpolation reports whether a captured Terraform string
// attribute still contains "${...}" interpolation syntax. The scanner has no
// HCL evaluator, so such a value can never be resolved to the real bucket
// name it represents at apply time.
func hasUnresolvedInterpolation(value string) bool {
	return strings.Contains(value, "${")
}

// scanTerraform scans Terraform files for S3 bucket references
func scanTerraform(filePath string) ([]Reference, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var refs []Reference
	scanner := bufio.NewScanner(file)
	lineNum := 0

	var inS3Resource bool
	var currentBucket string
	var currentResourceLine int

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check if entering S3 bucket resource
		if tfS3BucketResource.MatchString(trimmed) {
			inS3Resource = true
			currentResourceLine = lineNum
			currentBucket = ""
			continue
		}

		// Check if entering S3 object resource
		if tfS3ObjectResource.MatchString(trimmed) {
			inS3Resource = true
			currentResourceLine = lineNum
			currentBucket = ""
			continue
		}

		// Exit resource block
		if inS3Resource && trimmed == "}" {
			if currentBucket != "" {
				refs = append(refs, Reference{
					Bucket:  currentBucket,
					File:    filePath,
					Line:    currentResourceLine,
					Context: "terraform",
				})
			}
			inS3Resource = false
			currentBucket = ""
			continue
		}

		// Extract bucket name. An unresolved Terraform interpolation
		// (e.g. "${local.cluster_name}-landing") is never a real bucket
		// name -- the scanner has no HCL evaluator to resolve locals/vars,
		// and reporting the raw expression as if it were literal would
		// always produce a phantom MISSING_BUCKET finding. Skip it rather
		// than guess.
		if inS3Resource {
			if match := tfBucketNameAttr.FindStringSubmatch(trimmed); match != nil {
				if !hasUnresolvedInterpolation(match[1]) {
					currentBucket = match[1]
				}
			}
		}

		// Also check for s3:// URLs in any line
		if matches := s3URLPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if isPlaceholderBucketName(match[1]) {
					continue
				}
				refs = append(refs, Reference{
					Bucket:  match[1],
					Prefix:  match[2],
					File:    filePath,
					Line:    lineNum,
					Context: "terraform",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}
