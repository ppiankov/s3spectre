package scanner

import (
	"bufio"
	"os"
	"regexp"
)

var (
	// S3 URL patterns
	s3URLPattern  = regexp.MustCompile(`s3://([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])(?:/([^?\s"']+))?(?:\?versionId=([^\s"']+))?`)
	s3HTTPPattern = regexp.MustCompile(`https?://([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])\.s3(?:[.-]([a-z0-9-]+))?\.amazonaws\.com(?:/([^?\s"']+))?(?:\?versionId=([^\s"']+))?`)

	// Bucket name pattern (for env vars and config)
	bucketNamePattern = regexp.MustCompile(`(?i)(?:bucket|s3[-_]?bucket|s3[-_]?name)[\s:=]+['"]?([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])['"]?`)

	// Context detection patterns
	writeOpPattern = regexp.MustCompile(`(?i)(put|write|upload|store|save|create)`)
	readOpPattern  = regexp.MustCompile(`(?i)(get|read|download|fetch|retrieve|load)`)
	listOpPattern  = regexp.MustCompile(`(?i)(list|ls|scan|iterate)`)
)

// scanCode scans source code files using regex patterns
func scanCode(filePath string) ([]Reference, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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
					Bucket:    match[1],
					Prefix:    match[2],
					VersionID: match[3],
					File:      filePath,
					Line:      lineNum,
					Context:   detectContext(line),
				})
			}
		}

		// Check for HTTP(S) S3 URLs
		if matches := s3HTTPPattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				refs = append(refs, Reference{
					Bucket:    match[1],
					Prefix:    match[3],
					VersionID: match[4],
					File:      filePath,
					Line:      lineNum,
					Context:   detectContext(line),
				})
			}
		}

		// Check for bucket name references
		if matches := bucketNamePattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				// Avoid duplicates from URL patterns
				isDuplicate := false
				bucket := match[1]
				for _, ref := range refs {
					if ref.Bucket == bucket && ref.Line == lineNum {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					refs = append(refs, Reference{
						Bucket:  bucket,
						File:    filePath,
						Line:    lineNum,
						Context: detectContext(line),
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}

// detectContext tries to detect the type of S3 operation from the line
func detectContext(line string) string {
	// Write operations (check before read to catch "upload" before "load")
	if writeOpPattern.MatchString(line) {
		return "write"
	}

	// Read operations
	if readOpPattern.MatchString(line) {
		return "read"
	}

	// List operations
	if listOpPattern.MatchString(line) {
		return "list"
	}

	return "unknown"
}
