package scanner

import (
	"bufio"
	"os"
	"regexp"
)

// scanYAML scans YAML files for S3 bucket references
func scanYAML(filePath string) ([]Reference, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var refs []Reference
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// YAML-specific bucket patterns
	yamlBucketPattern := regexp.MustCompile(`(?i)(?:bucket|s3_bucket|s3Bucket):\s*['"]?([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])['"]?`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for s3:// URLs
		if matches := s3URLPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				refs = append(refs, Reference{
					Bucket:  match[1],
					Prefix:  match[2],
					File:    filePath,
					Line:    lineNum,
					Context: "yaml",
				})
			}
		}

		// Check for bucket: field
		if matches := yamlBucketPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				refs = append(refs, Reference{
					Bucket:  match[1],
					File:    filePath,
					Line:    lineNum,
					Context: "yaml",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}
