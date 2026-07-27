package scanner

import (
	"bufio"
	"os"
)

// scanJSON scans JSON files for S3 bucket references
func scanJSON(filePath string) ([]Reference, error) {
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

		// Check for s3:// URLs
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
					Context: "json",
				})
			}
		}

		// Check for HTTP(S) S3 URLs
		if matches := s3HTTPPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if isPlaceholderBucketName(match[1]) {
					continue
				}
				refs = append(refs, Reference{
					Bucket:  match[1],
					Prefix:  match[3],
					File:    filePath,
					Line:    lineNum,
					Context: "json",
				})
			}
		}

		// Check for bucket name pattern. Group 1 is the double-quoted
		// capture, group 2 the single-quoted one (JSON only ever produces
		// group 1, but handled the same way as scanCode for consistency).
		if matches := bucketNamePattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				bucket := match[1]
				if bucket == "" {
					bucket = match[2]
				}
				if isPlaceholderBucketName(bucket) {
					continue
				}
				refs = append(refs, Reference{
					Bucket:  bucket,
					File:    filePath,
					Line:    lineNum,
					Context: "json",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}
