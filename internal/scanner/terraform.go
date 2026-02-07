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

// scanTerraform scans Terraform files for S3 bucket references
func scanTerraform(filePath string) ([]Reference, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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

		// Extract bucket name
		if inS3Resource {
			if match := tfBucketNameAttr.FindStringSubmatch(trimmed); match != nil {
				currentBucket = match[1]
			}
		}

		// Also check for s3:// URLs in any line
		if matches := s3URLPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
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
