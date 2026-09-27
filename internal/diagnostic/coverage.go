package diagnostic

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"openapi-sdkgen/internal/failure"
)

// CoverageStatus records whether one registered analyzer examined all of its
// in-scope semantic surface.
type CoverageStatus string

const (
	CoverageComplete CoverageStatus = "complete"
	CoveragePartial  CoverageStatus = "partial"
	CoverageSkipped  CoverageStatus = "skipped"
)

// CoveragePrerequisite records one declared analyzer prerequisite.
type CoveragePrerequisite struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// AnalysisCoverage is the machine-readable discovery record for one analyzer.
// Phase-level completeness is derived from these records rather than asserted
// separately.
type AnalysisCoverage struct {
	Phase         Phase                  `json:"phase"`
	Analyzer      string                 `json:"analyzer"`
	Status        CoverageStatus         `json:"status"`
	Location      *Location              `json:"location,omitempty"`
	Target        string                 `json:"target,omitempty"`
	Route         string                 `json:"route,omitempty"`
	Operation     string                 `json:"operation,omitempty"`
	Capability    string                 `json:"capability,omitempty"`
	Scope         failure.Scope          `json:"scope,omitempty"`
	Prerequisites []CoveragePrerequisite `json:"prerequisites,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
	BlockedBy     []string               `json:"blockedBy,omitempty"`
}

func canonicalizeDiagnostics(values []Diagnostic) []Diagnostic {
	result := append([]Diagnostic(nil), values...)
	for index := range result {
		result[index].Related = normalizeLocations(result[index].Related)
		if result[index].ID == "" {
			source := result[index].IdentitySource
			if source == "" {
				source = stableSourceIdentity(result[index].Location.Source)
			}
			result[index].ID = stableDiagnosticID(result[index], source)
		}
	}
	result = Sort(result)
	write := 0
	seen := make(map[string]int, len(result))
	for _, value := range result {
		if previous, exists := seen[value.ID]; exists {
			merged := append(result[previous].Related, value.Related...)
			result[previous].Related = normalizeLocations(merged)
			continue
		}
		result[write] = value
		seen[value.ID] = write
		write++
	}
	return SanitizeSources(result[:write])
}

func stableDiagnosticID(value Diagnostic, source string) string {
	fields := []string{
		"diagnostic-v1",
		value.Code,
		string(value.Phase),
		value.Target,
		string(value.Scope),
		string(value.Effect),
		value.Rule,
		value.Action,
		value.Route,
		value.Operation,
		value.Capability,
		value.Location.Pointer,
		source,
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func stableSourceIdentity(source string) string {
	if source == "" {
		return ""
	}
	if parsed, err := url.Parse(source); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		return "url:" + parsed.String()
	}
	if filepath.IsAbs(source) {
		return "absolute:" + filepath.ToSlash(filepath.Clean(source))
	}
	if strings.ContainsAny(source, `/\`) || strings.HasPrefix(source, ".") {
		return "relative:" + filepath.ToSlash(filepath.Clean(source))
	}
	return "literal:" + source
}

func normalizeCoverage(values []AnalysisCoverage) []AnalysisCoverage {
	result := append([]AnalysisCoverage(nil), values...)
	sources := make([]string, 0, len(result))
	for index := range result {
		if result[index].Location != nil {
			location := *result[index].Location
			result[index].Location = &location
			if location.Source != "" {
				sources = append(sources, location.Source)
			}
		}
		result[index].Prerequisites = append([]CoveragePrerequisite(nil), result[index].Prerequisites...)
		sort.Slice(result[index].Prerequisites, func(i, j int) bool {
			left, right := result[index].Prerequisites[i], result[index].Prerequisites[j]
			if left.Name != right.Name {
				return left.Name < right.Name
			}
			if left.Available != right.Available {
				return left.Available && !right.Available
			}
			return left.Reason < right.Reason
		})
		result[index].BlockedBy = append([]string(nil), result[index].BlockedBy...)
		sort.Strings(result[index].BlockedBy)
		result[index].BlockedBy = deduplicateStrings(result[index].BlockedBy)
	}
	registry := NewSourceRegistry(sources)
	for index := range result {
		if result[index].Location != nil {
			result[index].Location.Source = registry.Display(result[index].Location.Source)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if rank := phaseRank(left.Phase) - phaseRank(right.Phase); rank != 0 {
			return rank < 0
		}
		if left.Analyzer != right.Analyzer {
			return left.Analyzer < right.Analyzer
		}
		leftSource, leftPointer := "", ""
		rightSource, rightPointer := "", ""
		if left.Location != nil {
			leftSource, leftPointer = left.Location.Source, left.Location.Pointer
		}
		if right.Location != nil {
			rightSource, rightPointer = right.Location.Source, right.Location.Pointer
		}
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		if leftPointer != rightPointer {
			return leftPointer < rightPointer
		}
		if left.Target != right.Target {
			return left.Target < right.Target
		}
		if left.Route != right.Route {
			return left.Route < right.Route
		}
		if left.Operation != right.Operation {
			return left.Operation < right.Operation
		}
		if left.Capability != right.Capability {
			return left.Capability < right.Capability
		}
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		if left.Status != right.Status {
			return coverageStatusRank(left.Status) < coverageStatusRank(right.Status)
		}
		return left.Reason < right.Reason
	})
	return result
}

func coverageStatusRank(value CoverageStatus) int {
	switch value {
	case CoverageComplete:
		return 0
	case CoveragePartial:
		return 1
	case CoverageSkipped:
		return 2
	default:
		return 3
	}
}
