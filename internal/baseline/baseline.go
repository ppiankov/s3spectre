package baseline

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/report"
)

// Finding is a flattened, identity-comparable issue from a scan or discovery.
type Finding struct {
	Type   string `json:"type"`
	Bucket string `json:"bucket"`
	Prefix string `json:"prefix,omitempty"`
}

func (f Finding) key() string {
	if f.Prefix != "" {
		return fmt.Sprintf("%s|%s|%s", f.Type, f.Bucket, f.Prefix)
	}
	return fmt.Sprintf("%s|%s", f.Type, f.Bucket)
}

// DiffResult holds the outcome of comparing current findings against a baseline.
type DiffResult struct {
	New       []Finding
	Resolved  []Finding
	Unchanged []Finding
}

// FlattenScanFindings converts a scan report into a flat finding list.
func FlattenScanFindings(data report.Data) []Finding {
	var findings []Finding
	for name, ba := range data.Buckets {
		if ba.Status != analyzer.StatusOK {
			findings = append(findings, Finding{Type: string(ba.Status), Bucket: name})
		}
		for _, pa := range ba.Prefixes {
			if pa.Status != analyzer.StatusOK {
				findings = append(findings, Finding{Type: string(pa.Status), Bucket: name, Prefix: pa.Prefix})
			}
		}
	}
	return findings
}

// FlattenDiscoveryFindings converts a discovery report into a flat finding list.
func FlattenDiscoveryFindings(data report.DiscoveryData) []Finding {
	var findings []Finding
	for name, bd := range data.Buckets {
		if bd.Status != analyzer.StatusOK {
			findings = append(findings, Finding{Type: string(bd.Status), Bucket: name})
		}
	}
	return findings
}

// LoadScanBaseline reads a previous scan JSON report and extracts findings.
func LoadScanBaseline(path string) ([]Finding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	var data report.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	return FlattenScanFindings(data), nil
}

// LoadDiscoveryBaseline reads a previous discovery JSON report and extracts findings.
func LoadDiscoveryBaseline(path string) ([]Finding, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	var data report.DiscoveryData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	return FlattenDiscoveryFindings(data), nil
}

// Diff compares current findings against a baseline.
func Diff(current, baseline []Finding) DiffResult {
	baseMap := make(map[string]struct{}, len(baseline))
	for _, f := range baseline {
		baseMap[f.key()] = struct{}{}
	}
	curMap := make(map[string]struct{}, len(current))
	for _, f := range current {
		curMap[f.key()] = struct{}{}
	}

	var result DiffResult
	for _, f := range current {
		if _, exists := baseMap[f.key()]; exists {
			result.Unchanged = append(result.Unchanged, f)
		} else {
			result.New = append(result.New, f)
		}
	}
	for _, f := range baseline {
		if _, exists := curMap[f.key()]; !exists {
			result.Resolved = append(result.Resolved, f)
		}
	}
	return result
}
