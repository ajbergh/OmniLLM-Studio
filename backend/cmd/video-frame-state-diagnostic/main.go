// Command video-frame-state-diagnostic compares browser/TypeScript and Go
// canonical visual FrameState diagnostics for the same saved Timeline v1 and
// output-frame sample set. It is parity tooling only; it never renders media or
// relaxes the fail-closed v1→v2 adapter.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	videopkg "github.com/ajbergh/omnillm-studio/internal/video"
)

const (
	reportVersion    = 1
	numericTolerance = 1e-9
)

type fixtureSample struct {
	Name       string `json:"name"`
	FrameIndex int64  `json:"frame_index"`
}

type fixtureFile struct {
	Samples []fixtureSample `json:"samples"`
}

type previewSample struct {
	Name       string                              `json:"name"`
	FrameIndex int64                               `json:"frame_index"`
	Diagnostic videopkg.VisualFrameStateDiagnostic `json:"diagnostic"`
}

type previewFile struct {
	Version            int             `json:"version"`
	Source             string          `json:"source"`
	DiagnosticContract string          `json:"diagnostic_contract"`
	TimelineSHA256     string          `json:"timeline_sha256"`
	SnapshotID         string          `json:"snapshot_id"`
	Samples            []previewSample `json:"samples"`
}

type sampleReport struct {
	Name               string   `json:"name"`
	FrameIndex         int64    `json:"frame_index"`
	Status             string   `json:"status"`
	PreviewAvailable   bool     `json:"preview_available"`
	BackendAvailable   bool     `json:"backend_available"`
	PreviewFingerprint string   `json:"preview_fingerprint,omitempty"`
	BackendFingerprint string   `json:"backend_fingerprint,omitempty"`
	ErrorCode          string   `json:"error_code,omitempty"`
	ErrorPath          string   `json:"error_path,omitempty"`
	MismatchPaths      []string `json:"mismatch_paths,omitempty"`
}

type comparisonReport struct {
	Version               int            `json:"version"`
	DiagnosticContract    string         `json:"diagnostic_contract"`
	NumericTolerance      float64        `json:"numeric_tolerance"`
	TimelineSHA256        string         `json:"timeline_sha256"`
	SnapshotID            string         `json:"snapshot_id"`
	Samples               []sampleReport `json:"samples"`
	AvailableCompared     int            `json:"available_compared"`
	MatchedUnavailable    int            `json:"matched_unavailable"`
	Mismatched            int            `json:"mismatched"`
	AllDiagnosticsMatched bool           `json:"all_diagnostics_matched"`
}

func main() {
	var timelinePath, fixturePath, previewPath, outputPath string
	flag.StringVar(&timelinePath, "timeline", "", "saved Timeline v1 JSON captured from the parity project")
	flag.StringVar(&fixturePath, "fixture", "", "parity fixture JSON containing frame samples")
	flag.StringVar(&previewPath, "preview", "", "browser/TypeScript FrameState diagnostic JSON")
	flag.StringVar(&outputPath, "output", "", "comparison report JSON")
	flag.Parse()
	if timelinePath == "" || fixturePath == "" || previewPath == "" || outputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: video-frame-state-diagnostic --timeline <timeline-v1.json> --fixture <fixture.json> --preview <preview.json> --output <report.json>")
		os.Exit(2)
	}

	var timeline videopkg.TimelineDocument
	mustReadJSON(timelinePath, &timeline)
	var fixture fixtureFile
	mustReadJSON(fixturePath, &fixture)
	var preview previewFile
	mustReadJSON(previewPath, &preview)

	report, err := compareDiagnostics(timeline, fixture, preview)
	if err != nil {
		fmt.Fprintf(os.Stderr, "frame-state diagnostic comparison: %v\n", err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal frame-state diagnostic report: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write frame-state diagnostic report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("FrameState diagnostics: %d available compared, %d matched unavailable, %d mismatched\n", report.AvailableCompared, report.MatchedUnavailable, report.Mismatched)
	if !report.AllDiagnosticsMatched {
		os.Exit(1)
	}
}

func compareDiagnostics(timeline videopkg.TimelineDocument, fixture fixtureFile, preview previewFile) (comparisonReport, error) {
	if preview.DiagnosticContract != videopkg.VisualFrameStateDiagnosticV1 {
		return comparisonReport{}, fmt.Errorf("preview diagnostic contract %q does not match %q", preview.DiagnosticContract, videopkg.VisualFrameStateDiagnosticV1)
	}
	if len(fixture.Samples) == 0 {
		return comparisonReport{}, fmt.Errorf("fixture contains no samples")
	}
	previewByKey := make(map[string]previewSample, len(preview.Samples))
	for _, sample := range preview.Samples {
		key := sampleKey(sample.Name, sample.FrameIndex)
		if _, exists := previewByKey[key]; exists {
			return comparisonReport{}, fmt.Errorf("duplicate preview diagnostic %s", key)
		}
		previewByKey[key] = sample
	}
	report := comparisonReport{
		Version:            reportVersion,
		DiagnosticContract: videopkg.VisualFrameStateDiagnosticV1,
		NumericTolerance:   numericTolerance,
		TimelineSHA256:     preview.TimelineSHA256,
		SnapshotID:         preview.SnapshotID,
		Samples:            make([]sampleReport, 0, len(fixture.Samples)),
	}
	for _, sample := range fixture.Samples {
		previewSample, ok := previewByKey[sampleKey(sample.Name, sample.FrameIndex)]
		if !ok {
			report.Samples = append(report.Samples, sampleReport{Name: sample.Name, FrameIndex: sample.FrameIndex, Status: "missing_preview", MismatchPaths: []string{"diagnostic"}})
			report.Mismatched++
			continue
		}
		backend := videopkg.EvaluateVisualFrameStateDiagnostic(timeline, sample.FrameIndex)
		entry := compareDiagnosticSample(sample, previewSample.Diagnostic, backend)
		report.Samples = append(report.Samples, entry)
		switch entry.Status {
		case "matched":
			report.AvailableCompared++
		case "matched_unavailable":
			report.MatchedUnavailable++
		default:
			report.Mismatched++
		}
	}
	report.AllDiagnosticsMatched = report.Mismatched == 0
	return report, nil
}

func compareDiagnosticSample(sample fixtureSample, preview, backend videopkg.VisualFrameStateDiagnostic) sampleReport {
	entry := sampleReport{
		Name:             sample.Name,
		FrameIndex:       sample.FrameIndex,
		PreviewAvailable: preview.Available,
		BackendAvailable: backend.Available,
	}
	mismatches := make([]string, 0)
	if preview.ContractVersion != backend.ContractVersion {
		mismatches = append(mismatches, "contract_version")
	}
	if preview.FrameIndex != sample.FrameIndex || backend.FrameIndex != sample.FrameIndex {
		mismatches = append(mismatches, "frame_index")
	}
	if preview.Available != backend.Available {
		mismatches = append(mismatches, "available")
		entry.Status = "mismatch"
		entry.MismatchPaths = mismatches
		return entry
	}
	if !preview.Available {
		if preview.Error == nil || backend.Error == nil {
			mismatches = append(mismatches, "error")
		} else {
			entry.ErrorCode = backend.Error.Code
			entry.ErrorPath = backend.Error.Path
			if preview.Error.Code != backend.Error.Code {
				mismatches = append(mismatches, "error.code")
			}
			if preview.Error.Path != backend.Error.Path {
				mismatches = append(mismatches, "error.path")
			}
		}
		if len(mismatches) == 0 {
			entry.Status = "matched_unavailable"
		} else {
			entry.Status = "mismatch"
			entry.MismatchPaths = mismatches
		}
		return entry
	}
	if preview.State == nil || backend.State == nil {
		entry.Status = "mismatch"
		entry.MismatchPaths = []string{"state"}
		return entry
	}
	entry.PreviewFingerprint = stateFingerprint(preview.State)
	entry.BackendFingerprint = stateFingerprint(backend.State)
	mismatches = append(mismatches, compareStateValues(preview.State, backend.State)...)
	if len(mismatches) == 0 {
		entry.Status = "matched"
	} else {
		entry.Status = "mismatch"
		entry.MismatchPaths = mismatches
	}
	return entry
}

func compareStateValues(left, right any) []string {
	leftValue := jsonValue(left)
	rightValue := jsonValue(right)
	mismatches := make([]string, 0)
	compareJSONValue("state", leftValue, rightValue, &mismatches)
	sort.Strings(mismatches)
	return mismatches
}

func compareJSONValue(path string, left, right any, mismatches *[]string) {
	switch l := left.(type) {
	case map[string]any:
		r, ok := right.(map[string]any)
		if !ok {
			*mismatches = append(*mismatches, path)
			return
		}
		keys := make(map[string]struct{})
		for key := range l {
			keys[key] = struct{}{}
		}
		for key := range r {
			keys[key] = struct{}{}
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			lv, lok := l[key]
			rv, rok := r[key]
			if !lok || !rok {
				*mismatches = append(*mismatches, path+"."+key)
				continue
			}
			compareJSONValue(path+"."+key, lv, rv, mismatches)
		}
	case []any:
		r, ok := right.([]any)
		if !ok || len(l) != len(r) {
			*mismatches = append(*mismatches, path)
			return
		}
		for index := range l {
			compareJSONValue(fmt.Sprintf("%s[%d]", path, index), l[index], r[index], mismatches)
		}
	case float64:
		r, ok := right.(float64)
		if !ok || math.Abs(l-r) > numericTolerance {
			*mismatches = append(*mismatches, path)
		}
	default:
		if fmt.Sprint(left) != fmt.Sprint(right) {
			*mismatches = append(*mismatches, path)
		}
	}
}

func stateFingerprint(value any) string {
	normalized := normalizeFingerprintValue(jsonValue(value))
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeFingerprintValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeFingerprintValue(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = normalizeFingerprintValue(item)
		}
		return out
	case float64:
		rounded := math.Round(typed/numericTolerance) * numericTolerance
		if math.Abs(rounded) < numericTolerance/2 {
			return float64(0)
		}
		return rounded
	default:
		return value
	}
}

func jsonValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

func sampleKey(name string, frameIndex int64) string {
	return strings.TrimSpace(name) + "@" + fmt.Sprint(frameIndex)
}

func mustReadJSON(path string, target any) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		os.Exit(1)
	}
	if err := json.Unmarshal(data, target); err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
		os.Exit(1)
	}
}
