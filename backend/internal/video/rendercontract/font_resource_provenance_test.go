package rendercontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fontResourceProvenanceFixture struct {
	Version                        int                               `json:"version"`
	Manifest                       RenderManifestV1                  `json:"manifest"`
	ExpectedFontResourceProvenance []EvaluatedFontResourceProvenance `json:"expected_font_resource_provenance"`
}

func loadFontResourceProvenanceFixture(t *testing.T) fontResourceProvenanceFixture {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "video-renderer", "test", "fixtures", "font-resource-provenance-v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font resource provenance fixture: %v", err)
	}
	var fixture fontResourceProvenanceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode font resource provenance fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestEvaluateFontResourceProvenanceMatchesSharedFixture(t *testing.T) {
	fixture := loadFontResourceProvenanceFixture(t)
	provenance, err := EvaluateFontResourceProvenance(fixture.Manifest)
	if err != nil {
		t.Fatalf("EvaluateFontResourceProvenance: %v", err)
	}
	if !reflect.DeepEqual(provenance, fixture.ExpectedFontResourceProvenance) {
		t.Fatalf("font resource provenance = %+v, want %+v", provenance, fixture.ExpectedFontResourceProvenance)
	}
}

func TestEvaluateFontResourceProvenanceAllowsNoPackagedFonts(t *testing.T) {
	fixture := loadFontResourceProvenanceFixture(t)
	manifest := cloneRenderManifest(t, fixture.Manifest)
	manifest.FontResources = nil
	provenance, err := EvaluateFontResourceProvenance(manifest)
	if err != nil {
		t.Fatalf("EvaluateFontResourceProvenance: %v", err)
	}
	if len(provenance) != 0 {
		t.Fatalf("font resource provenance = %+v, want no resources", provenance)
	}
}

func TestEvaluateFontResourceProvenanceRejectsAmbiguousOrMalformedResources(t *testing.T) {
	fixture := loadFontResourceProvenanceFixture(t)
	tests := []struct {
		name   string
		mutate func(*RenderManifestV1)
		want   string
	}{
		{
			name:   "noncanonical id",
			mutate: func(manifest *RenderManifestV1) { manifest.FontResources[0].FontResourceID = "Inter 700" },
			want:   `font resource provenance font resource id "Inter 700" must use lowercase ASCII letters, digits, dots, underscores, or hyphens`,
		},
		{
			name: "duplicate id",
			mutate: func(manifest *RenderManifestV1) {
				manifest.FontResources[1].FontResourceID = manifest.FontResources[0].FontResourceID
			},
			want: `font resource provenance has duplicate font resource "inter-700-italic"`,
		},
		{
			name:   "surrounding family whitespace",
			mutate: func(manifest *RenderManifestV1) { manifest.FontResources[0].FontFamily = " Inter" },
			want:   `font resource provenance font resource "inter-700-italic" font family " Inter" must not have surrounding whitespace`,
		},
		{
			name:   "unsupported style",
			mutate: func(manifest *RenderManifestV1) { manifest.FontResources[0].FontStyle = "oblique" },
			want:   `font resource provenance font resource "inter-700-italic" has unsupported font style "oblique"`,
		},
		{
			name:   "unsafe staged path",
			mutate: func(manifest *RenderManifestV1) { manifest.FontResources[0].StagedPath = "../system-font.woff2" },
			want:   `font resource provenance font resource "inter-700-italic" staged path must be a clean relative POSIX path`,
		},
		{
			name: "invalid hash",
			mutate: func(manifest *RenderManifestV1) {
				manifest.FontResources[0].FileSHA256 = strings.ToUpper(manifest.FontResources[0].FileSHA256)
			},
			want: `font resource provenance font resource "inter-700-italic" has an invalid file_sha256`,
		},
		{
			name:   "empty bytes",
			mutate: func(manifest *RenderManifestV1) { manifest.FontResources[0].SizeBytes = 0 },
			want:   `font resource provenance font resource "inter-700-italic" size_bytes must be positive`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := cloneRenderManifest(t, fixture.Manifest)
			test.mutate(&manifest)
			_, err := EvaluateFontResourceProvenance(manifest)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
