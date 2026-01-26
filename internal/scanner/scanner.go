package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// RepoScanner scans a repository for S3 references
type RepoScanner struct {
	repoPath string
}

// NewRepoScanner creates a new repository scanner
func NewRepoScanner(repoPath string) *RepoScanner {
	return &RepoScanner{
		repoPath: repoPath,
	}
}

// Scan scans the repository and returns all S3 references found
func (s *RepoScanner) Scan(ctx context.Context) ([]Reference, error) {
	var allRefs []Reference
	bucketsSeen := make(map[string]bool) // Deduplicate buckets

	// Walk through repository
	err := filepath.Walk(s.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip binary files and large files
		if info.Size() > 10*1024*1024 { // Skip files > 10MB
			return nil
		}

		// Scan file based on extension
		refs, err := s.scanFile(path)
		if err != nil {
			// Log error but continue
			return nil
		}

		for _, ref := range refs {
			key := ref.Bucket + "|" + ref.Prefix
			if !bucketsSeen[key] {
				allRefs = append(allRefs, ref)
				bucketsSeen[key] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return allRefs, nil
}

// scanFile scans a single file for S3 references
func (s *RepoScanner) scanFile(filePath string) ([]Reference, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	basename := strings.ToLower(filepath.Base(filePath))

	// Choose scanner based on file type
	switch {
	case ext == ".tf" || ext == ".hcl":
		return scanTerraform(filePath)
	case ext == ".yaml" || ext == ".yml":
		return scanYAML(filePath)
	case ext == ".json":
		return scanJSON(filePath)
	case basename == ".env" || strings.HasSuffix(basename, ".env"):
		return scanEnv(filePath)
	case ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".go" || ext == ".java" || ext == ".sh":
		return scanCode(filePath)
	default:
		return nil, nil
	}
}
