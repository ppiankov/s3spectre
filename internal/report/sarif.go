package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ppiankov/s3spectre/internal/analyzer"
	"github.com/ppiankov/s3spectre/internal/scanner"
)

const (
	sarifSchema  = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifVersion = "2.1.0"

	sarifRuleMissingBucket  = "s3spectre/MISSING_BUCKET"
	sarifRuleMissingPrefix  = "s3spectre/MISSING_PREFIX"
	sarifRuleStalePrefix    = "s3spectre/STALE_PREFIX"
	sarifRuleUnusedBucket   = "s3spectre/UNUSED_BUCKET"
	sarifRuleVersionSprawl  = "s3spectre/VERSION_SPRAWL"
	sarifRuleLifecycleGap   = "s3spectre/LIFECYCLE_GAP"
	sarifRulePublicBucket   = "s3spectre/PUBLIC_BUCKET"
	sarifRuleNoEncryption   = "s3spectre/NO_ENCRYPTION"
	sarifRuleInactiveBucket = "s3spectre/INACTIVE_BUCKET"
	sarifRuleRiskyBucket    = "s3spectre/RISKY_BUCKET"
)

type SARIFReporter struct {
	writer io.Writer
}

func NewSARIFReporter(w io.Writer) *SARIFReporter {
	return &SARIFReporter{writer: w}
}

type sarifLog struct {
	Schema  string     `json:"$schema,omitempty"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version,omitempty"`
	Rules   []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name,omitempty"`
	ShortDescription sarifMessage `json:"shortDescription,omitempty"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level,omitempty"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation *sarifPhysicalLocation `json:"physicalLocation,omitempty"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine,omitempty"`
}

type sarifRuleMeta struct {
	Name        string
	Description string
	Level       string
}

var sarifRules = map[string]sarifRuleMeta{
	sarifRuleMissingBucket: {
		Name:        "MissingBucket",
		Description: "Bucket referenced in code but does not exist in AWS",
		Level:       "warning",
	},
	sarifRuleMissingPrefix: {
		Name:        "MissingPrefix",
		Description: "Prefix referenced in code but no objects were found",
		Level:       "warning",
	},
	sarifRuleStalePrefix: {
		Name:        "StalePrefix",
		Description: "Prefix has not been modified recently",
		Level:       "note",
	},
	sarifRuleUnusedBucket: {
		Name:        "UnusedBucket",
		Description: "Bucket appears unused",
		Level:       "note",
	},
	sarifRuleVersionSprawl: {
		Name:        "VersionSprawl",
		Description: "Versioning enabled without lifecycle rules",
		Level:       "note",
	},
	sarifRuleLifecycleGap: {
		Name:        "LifecycleGap",
		Description: "Lifecycle rules are missing for the bucket",
		Level:       "note",
	},
	sarifRulePublicBucket: {
		Name:        "PublicBucket",
		Description: "Bucket is publicly accessible",
		Level:       "error",
	},
	sarifRuleNoEncryption: {
		Name:        "NoEncryption",
		Description: "Bucket does not have default encryption enabled",
		Level:       "warning",
	},
	sarifRuleInactiveBucket: {
		Name:        "InactiveBucket",
		Description: "Bucket has been inactive for an extended period",
		Level:       "warning",
	},
	sarifRuleRiskyBucket: {
		Name:        "RiskyBucket",
		Description: "Bucket risk score exceeds the configured threshold",
		Level:       "warning",
	},
}

func (r *SARIFReporter) Generate(data Data) error {
	bucketRefs, prefixRefs := collectReferences(data.References)

	var results []sarifResult
	usedRules := make(map[string]sarifRule)

	bucketNames := make([]string, 0, len(data.Buckets))
	for bucket := range data.Buckets {
		bucketNames = append(bucketNames, bucket)
	}
	sort.Strings(bucketNames)

	for _, bucket := range bucketNames {
		analysis := data.Buckets[bucket]
		if analysis == nil {
			continue
		}

		switch analysis.Status {
		case analyzer.StatusMissingBucket:
			message := fallbackMessage(analysis.Message, sarifRuleMissingBucket)
			locations := locationsWithFallback(bucketRefs[bucket], s3URI(bucket))
			results = appendResult(results, usedRules, sarifRuleMissingBucket, message, locations)
		case analyzer.StatusUnusedBucket:
			message := fallbackMessage(analysis.Message, sarifRuleUnusedBucket)
			locations := locationsWithFallback(bucketRefs[bucket], s3URI(bucket))
			results = appendResult(results, usedRules, sarifRuleUnusedBucket, message, locations)
		case analyzer.StatusVersionSprawl:
			message := fallbackMessage(analysis.Message, sarifRuleVersionSprawl)
			locations := locationsWithFallback(bucketRefs[bucket], s3URI(bucket))
			results = appendResult(results, usedRules, sarifRuleVersionSprawl, message, locations)
		case analyzer.StatusLifecycleMisconfig:
			message := fallbackMessage(analysis.Message, sarifRuleLifecycleGap)
			locations := locationsWithFallback(bucketRefs[bucket], s3URI(bucket))
			results = appendResult(results, usedRules, sarifRuleLifecycleGap, message, locations)
		}

		if len(analysis.Prefixes) == 0 {
			continue
		}

		prefixes := make([]analyzer.PrefixAnalysis, len(analysis.Prefixes))
		copy(prefixes, analysis.Prefixes)
		sort.Slice(prefixes, func(i, j int) bool {
			return prefixes[i].Prefix < prefixes[j].Prefix
		})

		for _, prefix := range prefixes {
			switch prefix.Status {
			case analyzer.StatusMissingPrefix:
				message := fallbackMessage(prefix.Message, sarifRuleMissingPrefix)
				locations := locationsWithFallback(prefixRefs[bucket][prefix.Prefix], s3URI(bucket, prefix.Prefix))
				results = appendResult(results, usedRules, sarifRuleMissingPrefix, message, locations)
			case analyzer.StatusStalePrefix:
				message := fallbackMessage(prefix.Message, sarifRuleStalePrefix)
				locations := locationsWithFallback(prefixRefs[bucket][prefix.Prefix], s3URI(bucket, prefix.Prefix))
				results = appendResult(results, usedRules, sarifRuleStalePrefix, message, locations)
			}
		}
	}

	return r.writeSARIF(data.Tool, data.Version, results, usedRules)
}

func (r *SARIFReporter) GenerateDiscovery(data DiscoveryData) error {
	var results []sarifResult
	usedRules := make(map[string]sarifRule)

	bucketNames := make([]string, 0, len(data.Buckets))
	for bucket := range data.Buckets {
		bucketNames = append(bucketNames, bucket)
	}
	sort.Strings(bucketNames)

	for _, bucket := range bucketNames {
		discovery := data.Buckets[bucket]
		if discovery == nil {
			continue
		}

		locations := locationsWithFallback(nil, s3URI(bucket))

		switch discovery.Status {
		case analyzer.StatusUnusedBucket:
			message := discoveryStatusMessage(discovery, "Bucket appears unused")
			results = appendResult(results, usedRules, sarifRuleUnusedBucket, message, locations)
		case analyzer.StatusRisky:
			message := discoveryStatusMessage(discovery, "Bucket risk score exceeds the threshold")
			results = appendResult(results, usedRules, sarifRuleRiskyBucket, message, locations)
		case analyzer.StatusInactive:
			message := discoveryStatusMessage(discovery, "Bucket has been inactive")
			results = appendResult(results, usedRules, sarifRuleInactiveBucket, message, locations)
		case analyzer.StatusVersionSprawl:
			message := discoveryStatusMessage(discovery, "Versioning enabled without lifecycle rules")
			results = appendResult(results, usedRules, sarifRuleVersionSprawl, message, locations)
		}

		if data.Config.CheckPublicAccess && discovery.BucketInfo != nil && discovery.BucketInfo.PublicAccess != nil && discovery.BucketInfo.PublicAccess.IsPublic {
			message := fallbackMessage("", sarifRulePublicBucket)
			results = appendResult(results, usedRules, sarifRulePublicBucket, message, locations)
		}

		if data.Config.CheckEncryption && discovery.BucketInfo != nil && discovery.BucketInfo.Encryption != nil && !discovery.BucketInfo.Encryption.Enabled {
			message := fallbackMessage("", sarifRuleNoEncryption)
			results = appendResult(results, usedRules, sarifRuleNoEncryption, message, locations)
		}
	}

	return r.writeSARIF(data.Tool, data.Version, results, usedRules)
}

func (r *SARIFReporter) writeSARIF(toolName, toolVersion string, results []sarifResult, usedRules map[string]sarifRule) error {
	ruleIDs := make([]string, 0, len(usedRules))
	for id := range usedRules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)

	rules := make([]sarifRule, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		rules = append(rules, usedRules[id])
	}

	log := sarifLog{
		Schema:  sarifSchema,
		Version: sarifVersion,
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:    toolName,
					Version: toolVersion,
					Rules:   rules,
				},
			},
			Results: results,
		}},
	}

	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(log)
}

func appendResult(results []sarifResult, usedRules map[string]sarifRule, ruleID, message string, locations []sarifLocation) []sarifResult {
	rule := sarifRule{ID: ruleID}
	level := "warning"
	if meta, ok := sarifRules[ruleID]; ok {
		rule.Name = meta.Name
		rule.ShortDescription = sarifMessage{Text: meta.Description}
		level = meta.Level
	}
	if message == "" {
		message = rule.ShortDescription.Text
	}
	if _, exists := usedRules[ruleID]; !exists {
		usedRules[ruleID] = rule
	}

	results = append(results, sarifResult{
		RuleID:    ruleID,
		Level:     level,
		Message:   sarifMessage{Text: message},
		Locations: locations,
	})

	return results
}

func fallbackMessage(message, ruleID string) string {
	if message != "" {
		return message
	}
	if meta, ok := sarifRules[ruleID]; ok {
		return meta.Description
	}
	return message
}

func collectReferences(refs []scanner.Reference) (map[string][]scanner.Reference, map[string]map[string][]scanner.Reference) {
	bucketRefs := make(map[string][]scanner.Reference)
	prefixRefs := make(map[string]map[string][]scanner.Reference)
	for _, ref := range refs {
		bucketRefs[ref.Bucket] = append(bucketRefs[ref.Bucket], ref)
		if ref.Prefix == "" {
			continue
		}
		if _, ok := prefixRefs[ref.Bucket]; !ok {
			prefixRefs[ref.Bucket] = make(map[string][]scanner.Reference)
		}
		prefixRefs[ref.Bucket][ref.Prefix] = append(prefixRefs[ref.Bucket][ref.Prefix], ref)
	}
	return bucketRefs, prefixRefs
}

func locationsWithFallback(refs []scanner.Reference, fallbackURI string) []sarifLocation {
	locations := buildLocationsFromRefs(refs)
	if len(locations) > 0 {
		return locations
	}
	if fallbackURI == "" {
		return nil
	}
	return []sarifLocation{{
		PhysicalLocation: &sarifPhysicalLocation{
			ArtifactLocation: sarifArtifactLocation{URI: fallbackURI},
		},
	}}
}

func buildLocationsFromRefs(refs []scanner.Reference) []sarifLocation {
	type locationKey struct {
		file string
		line int
	}
	seen := make(map[locationKey]struct{})
	keys := make([]locationKey, 0, len(refs))
	for _, ref := range refs {
		if ref.File == "" {
			continue
		}
		key := locationKey{file: ref.File, line: ref.Line}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].file == keys[j].file {
			return keys[i].line < keys[j].line
		}
		return keys[i].file < keys[j].file
	})

	locations := make([]sarifLocation, 0, len(keys))
	for _, key := range keys {
		physical := &sarifPhysicalLocation{
			ArtifactLocation: sarifArtifactLocation{URI: key.file},
		}
		if key.line > 0 {
			physical.Region = &sarifRegion{StartLine: key.line}
		}
		locations = append(locations, sarifLocation{PhysicalLocation: physical})
	}
	return locations
}

func s3URI(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		cleaned = append(cleaned, strings.TrimPrefix(part, "/"))
	}
	if len(cleaned) == 0 {
		return ""
	}
	return "s3://" + strings.Join(cleaned, "/")
}

func discoveryStatusMessage(discovery *analyzer.BucketDiscovery, base string) string {
	message := base
	if discovery != nil {
		if discovery.RiskScore > 0 {
			switch discovery.Status {
			case analyzer.StatusRisky:
				message = fmt.Sprintf("Bucket risk score: %d", discovery.RiskScore)
			case analyzer.StatusUnusedBucket:
				message = fmt.Sprintf("Bucket appears unused (risk score: %d)", discovery.RiskScore)
			case analyzer.StatusInactive:
				message = fmt.Sprintf("Bucket inactive (risk score: %d)", discovery.RiskScore)
			case analyzer.StatusVersionSprawl:
				message = fmt.Sprintf("Versioning enabled without lifecycle rules (risk score: %d)", discovery.RiskScore)
			}
		}
		if discovery.BucketInfo != nil && discovery.Status == analyzer.StatusInactive && discovery.BucketInfo.DaysSinceActivity > 0 {
			message = fmt.Sprintf("No activity for %d days", discovery.BucketInfo.DaysSinceActivity)
		}
		if len(discovery.RiskFactors) > 0 {
			message = fmt.Sprintf("%s. Factors: %s", message, strings.Join(discovery.RiskFactors, "; "))
		}
	}
	return message
}
