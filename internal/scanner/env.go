package scanner

import (
	"bufio"
	"os"
	"regexp"
)

var (
	// Environment variable patterns for S3 buckets
	envBucketPattern = regexp.MustCompile(`(?i)(?:S3_BUCKET|BUCKET|AWS_BUCKET|BUCKET_NAME)=['"]?([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])['"]?`)
)

// scanEnv scans environment files for S3 bucket references
func scanEnv(filePath string) ([]Reference, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var refs []Reference
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comments
		if len(line) > 0 && line[0] == '#' {
			continue
		}

		// Check for s3:// URLs
		if matches := s3URLPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				refs = append(refs, Reference{
					Bucket:  match[1],
					Prefix:  match[2],
					File:    filePath,
					Line:    lineNum,
					Context: "env",
				})
			}
		}

		// Check for environment variable bucket references
		if matches := envBucketPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				refs = append(refs, Reference{
					Bucket:  match[1],
					File:    filePath,
					Line:    lineNum,
					Context: "env",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}
