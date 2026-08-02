package scanner

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var (
	// S3 URL patterns
	s3URLPattern  = regexp.MustCompile(`s3://([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])(?:/([^?\s"']+))?(?:\?versionId=([^\s"']+))?`)
	s3HTTPPattern = regexp.MustCompile(`https?://([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])\.s3(?:[.-]([a-z0-9-]+))?\.amazonaws\.com(?:/([^?\s"']+))?(?:\?versionId=([^\s"']+))?`)

	// Bucket name pattern (for env vars and config). Requires the value to be
	// a quoted string literal (Go's RE2 engine has no backreferences, so
	// double- and single-quoted forms are two separate alternatives rather
	// than one pattern with a matching-quote backreference) -- an unquoted
	// match would be a code expression (variable, attribute access, function
	// call), not a literal bucket name.
	//
	// The leading (?:^|[^a-z]) requires the keyword not be immediately
	// preceded by a letter, so it rejects a keyword embedded as a bare
	// camelCase suffix (e.g. "opCreateBucket", "ErrCodeNoSuchBucket" -- AWS
	// SDK operation/error-code constants, not bucket references) while still
	// matching a snake_case/kebab-case identifier like "other_bucket" or
	// "my-bucket-name" (preceded by "_"/"-", which are non-letters). Go's
	// RE2 has no lookbehind, so the preceding character is consumed as part
	// of the match rather than asserted -- this doesn't affect the numbered
	// capture groups below, since it's a non-capturing group.
	bucketNamePattern = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:bucket|s3[-_]?bucket|s3[-_]?name)[\s:=]+(?:"([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])"|'([a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9])')`)

	// Context detection patterns
	writeOpPattern = regexp.MustCompile(`(?i)(put|write|upload|store|save|create)`)
	readOpPattern  = regexp.MustCompile(`(?i)(get|read|download|fetch|retrieve|load)`)
	listOpPattern  = regexp.MustCompile(`(?i)(list|ls|scan|iterate)`)
)

// placeholderBucketNames lists generic bucket-name tokens commonly used as
// illustrative examples in documentation, docstrings, and comments (e.g.
// "s3://bucket/key" as generic usage-example text). Real S3 bucket names are
// globally unique across all of AWS, so no genuine production bucket is
// actually named one of these bare placeholder words -- a match is always a
// false positive, never a real reference worth reporting.
var placeholderBucketNames = map[string]bool{
	"bucket":           true,
	"my-bucket":        true,
	"your-bucket":      true,
	"bucket-name":      true,
	"example-bucket":   true,
	"your-bucket-name": true,
	"my-bucket-name":   true,
	// "doc" is not a documentation-example placeholder like the entries
	// above -- it's AWS's own hardcoded S3 API XML namespace URI,
	// doc.s3.amazonaws.com/2006-03-01/, which appears verbatim throughout
	// the AWS SDK's own source and doc comments and happens to share
	// s3HTTPPattern's virtual-hosted-style URL shape.
	"doc": true,
	// "default-bucket" is a generic hardcoded default/zero-value seen in a
	// real SDK-wrapper library's options struct, not an illustrative
	// documentation example.
	"default-bucket": true,
}

// isPlaceholderBucketName reports whether name is a common documentation
// placeholder rather than a real bucket name reference.
func isPlaceholderBucketName(name string) bool {
	return placeholderBucketNames[strings.ToLower(name)]
}

// scanCode scans source code files using regex patterns
func scanCode(filePath string) ([]Reference, error) {
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
				if isPlaceholderBucketName(match[1]) {
					continue
				}
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

		// Check for bucket name references. Group 1 is the double-quoted
		// capture, group 2 the single-quoted one -- exactly one is non-empty
		// for any match, since the pattern requires one quote style or the
		// other (never neither, per the WO-49 fix).
		if matches := bucketNamePattern.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				bucket := match[1]
				if bucket == "" {
					bucket = match[2]
				}
				if isPlaceholderBucketName(bucket) {
					continue
				}
				// Avoid duplicates from URL patterns
				isDuplicate := false
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
