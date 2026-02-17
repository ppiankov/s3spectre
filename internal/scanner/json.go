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
				refs = append(refs, Reference{
					Bucket:  match[1],
					Prefix:  match[3],
					File:    filePath,
					Line:    lineNum,
					Context: "json",
				})
			}
		}

		// Check for bucket name pattern
		if matches := bucketNamePattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				refs = append(refs, Reference{
					Bucket:  match[1],
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
