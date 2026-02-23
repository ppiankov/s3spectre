package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ppiankov/s3spectre/internal/analyzer"
)

// spectre/v1 envelope types

type spectreEnvelope struct {
	Schema    string           `json:"schema"`
	Tool      string           `json:"tool"`
	Version   string           `json:"version"`
	Timestamp string           `json:"timestamp"`
	Target    spectreTarget    `json:"target"`
	Findings  []spectreFinding `json:"findings"`
	Summary   spectreSummary   `json:"summary"`
}

type spectreTarget struct {
	Type    string `json:"type"`
	URIHash string `json:"uri_hash"`
}

type spectreFinding struct {
	ID       string         `json:"id"`
	Severity string         `json:"severity"`
	Location string         `json:"location"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type spectreSummary struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
	Info   int `json:"info"`
}

// HashRegion produces a sha256 hash of an AWS region/profile for target identification.
func HashRegion(region, profile string) string {
	input := region + ":" + profile
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("sha256:%x", h)
}

// SpectreHubReporter generates spectre/v1 JSON envelope output.
type SpectreHubReporter struct {
	writer io.Writer
}

// NewSpectreHubReporter creates a new SpectreHub reporter.
func NewSpectreHubReporter(w io.Writer) *SpectreHubReporter {
	return &SpectreHubReporter{writer: w}
}

// Generate writes scan results as a spectre/v1 envelope.
func (r *SpectreHubReporter) Generate(data Data) error {
	envelope := spectreEnvelope{
		Schema:    "spectre/v1",
		Tool:      "s3spectre",
		Version:   data.Version,
		Timestamp: data.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
		Target: spectreTarget{
			Type:    "s3",
			URIHash: HashRegion(data.Config.AWSRegion, data.Config.AWSProfile),
		},
	}

	for name, bucket := range data.Buckets {
		if bucket.Status == analyzer.StatusOK {
			continue
		}
		severity := scanStatusSeverity(bucket.Status)
		envelope.Findings = append(envelope.Findings, spectreFinding{
			ID:       string(bucket.Status),
			Severity: severity,
			Location: name,
			Message:  bucket.Message,
		})
		countSeverity(&envelope.Summary, severity)

		// Prefix-level findings
		for _, p := range bucket.Prefixes {
			if p.Status == analyzer.StatusOK {
				continue
			}
			psev := scanStatusSeverity(p.Status)
			loc := name + "/" + p.Prefix
			envelope.Findings = append(envelope.Findings, spectreFinding{
				ID:       string(p.Status),
				Severity: psev,
				Location: loc,
				Message:  p.Message,
			})
			countSeverity(&envelope.Summary, psev)
		}
	}

	envelope.Summary.Total = len(envelope.Findings)
	if envelope.Findings == nil {
		envelope.Findings = []spectreFinding{}
	}

	enc := json.NewEncoder(r.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

// GenerateDiscovery writes discovery results as a spectre/v1 envelope.
func (r *SpectreHubReporter) GenerateDiscovery(data DiscoveryData) error {
	envelope := spectreEnvelope{
		Schema:    "spectre/v1",
		Tool:      "s3spectre",
		Version:   data.Version,
		Timestamp: data.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
		Target: spectreTarget{
			Type:    "s3",
			URIHash: HashRegion("", data.Config.AWSProfile),
		},
	}

	for name, bucket := range data.Buckets {
		if bucket.Status == analyzer.StatusOK {
			continue
		}
		severity := discoveryStatusSeverity(bucket.Status, bucket.RiskScore)
		envelope.Findings = append(envelope.Findings, spectreFinding{
			ID:       string(bucket.Status),
			Severity: severity,
			Location: name,
			Message:  fmt.Sprintf("risk score %d: %v", bucket.RiskScore, bucket.RiskFactors),
			Metadata: map[string]any{
				"risk_score":      bucket.RiskScore,
				"region":          bucket.Region,
				"recommendations": bucket.Recommendations,
			},
		})
		countSeverity(&envelope.Summary, severity)
	}

	envelope.Summary.Total = len(envelope.Findings)
	if envelope.Findings == nil {
		envelope.Findings = []spectreFinding{}
	}

	enc := json.NewEncoder(r.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func scanStatusSeverity(status analyzer.Status) string {
	switch status {
	case analyzer.StatusMissingBucket:
		return "high"
	case analyzer.StatusUnusedBucket, analyzer.StatusLifecycleMisconfig:
		return "medium"
	case analyzer.StatusMissingPrefix, analyzer.StatusVersionSprawl:
		return "medium"
	case analyzer.StatusStalePrefix:
		return "low"
	default:
		return "info"
	}
}

func discoveryStatusSeverity(status analyzer.Status, riskScore int) string {
	switch status {
	case analyzer.StatusRisky:
		if riskScore >= 80 {
			return "high"
		}
		return "medium"
	case analyzer.StatusUnusedBucket:
		return "medium"
	case analyzer.StatusInactive:
		return "low"
	case analyzer.StatusVersionSprawl:
		return "low"
	default:
		return "info"
	}
}

func countSeverity(s *spectreSummary, severity string) {
	switch severity {
	case "high":
		s.High++
	case "medium":
		s.Medium++
	case "low":
		s.Low++
	case "info":
		s.Info++
	}
}
