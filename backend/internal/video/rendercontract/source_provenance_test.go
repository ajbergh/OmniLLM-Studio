package rendercontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type sourceProvenanceFixture struct {
	Version                  int                       `json:"version"`
	Manifest                 RenderManifestV1          `json:"manifest"`
	ExpectedSourceProvenance EvaluatedSourceProvenance `json:"expected_source_provenance"`
	ExpectedModelMatrix      Matrix4                   `json:"expected_model_matrix"`
}

func loadSourceProvenanceFixture(t *testing.T) sourceProvenanceFixture {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "video-renderer", "test", "fixtures", "source-provenance-v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source provenance fixture: %v", err)
	}
	var fixture sourceProvenanceFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode source provenance fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestEvaluateSourceProvenanceMatchesSharedFixture(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	provenance, err := EvaluateSourceProvenance(fixture.Manifest)
	if err != nil {
		t.Fatalf("EvaluateSourceProvenance: %v", err)
	}
	if !reflect.DeepEqual(provenance, []EvaluatedSourceProvenance{fixture.ExpectedSourceProvenance}) {
		t.Fatalf("source provenance = %+v, want %+v", provenance, fixture.ExpectedSourceProvenance)
	}
}

func TestEvaluateVisualFrameStateForRenderManifestUsesSourceProvenance(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	state, err := EvaluateVisualFrameStateForRenderManifest(fixture.Manifest, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameStateForRenderManifest: %v", err)
	}
	if !state.Authoritative || len(state.Unresolved) != 0 || len(state.Layers) != 1 {
		t.Fatalf("state = %+v", state)
	}
	layer := state.Layers[0]
	if !reflect.DeepEqual(layer.SourceProvenance, &fixture.ExpectedSourceProvenance) {
		t.Fatalf("source provenance = %+v, want %+v", layer.SourceProvenance, fixture.ExpectedSourceProvenance)
	}
	if layer.ContentBounds == nil || !reflect.DeepEqual(*layer.ContentBounds, fixture.ExpectedSourceProvenance.SourceBounds) {
		t.Fatalf("content bounds = %+v, want %+v", layer.ContentBounds, fixture.ExpectedSourceProvenance.SourceBounds)
	}
	if layer.MediaGeometry == nil || !reflect.DeepEqual(layer.MediaGeometry.SourceBounds, fixture.ExpectedSourceProvenance.SourceBounds) {
		t.Fatalf("media geometry = %+v", layer.MediaGeometry)
	}
	if !reflect.DeepEqual(layer.ModelMatrix, fixture.ExpectedModelMatrix) {
		t.Fatalf("model matrix = %v, want %v", layer.ModelMatrix, fixture.ExpectedModelMatrix)
	}
}

func TestEvaluateVisualFrameStateForRenderManifestKeepsExplicitContentBoundsAuthoritative(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	manifest := cloneRenderManifest(t, fixture.Manifest)
	explicitBounds := TimelineV2ContentBounds{X: 5, Y: 6, Width: 50, Height: 25}
	manifest.Timeline.Tracks[0].Clips[0].ContentBounds = &explicitBounds
	state, err := EvaluateVisualFrameStateForRenderManifest(manifest, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameStateForRenderManifest: %v", err)
	}
	layer := state.Layers[0]
	if layer.ContentBounds == nil || !reflect.DeepEqual(*layer.ContentBounds, explicitBounds) {
		t.Fatalf("content bounds = %+v, want %+v", layer.ContentBounds, explicitBounds)
	}
	if layer.MediaGeometry == nil || !reflect.DeepEqual(layer.MediaGeometry.SourceBounds, explicitBounds) {
		t.Fatalf("media geometry = %+v, want explicit bounds %+v", layer.MediaGeometry, explicitBounds)
	}
	if !reflect.DeepEqual(layer.SourceProvenance, &fixture.ExpectedSourceProvenance) {
		t.Fatalf("source provenance = %+v, want %+v", layer.SourceProvenance, fixture.ExpectedSourceProvenance)
	}
}

func TestEvaluateVisualFrameStateForRenderManifestLeavesMissingSourceProbeExplicit(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	manifest := cloneRenderManifest(t, fixture.Manifest)
	manifest.Assets = []RenderManifestAsset{}
	state, err := EvaluateVisualFrameStateForRenderManifest(manifest, 0)
	if err != nil {
		t.Fatalf("EvaluateVisualFrameStateForRenderManifest: %v", err)
	}
	if state.Authoritative || !reflect.DeepEqual(state.Unresolved, []string{"visual-clip:content_bounds_for_anchor", "visual-clip:media_geometry:source_provenance"}) {
		t.Fatalf("state unresolved = %v", state.Unresolved)
	}
	if len(state.Layers) != 1 || state.Layers[0].ContentBounds != nil || state.Layers[0].SourceProvenance != nil || state.Layers[0].MediaGeometry != nil {
		t.Fatalf("missing source probe fabricated state: %+v", state.Layers)
	}
}

func TestEvaluateVisualFrameStateForRenderManifestRejectsUnboundSource(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	manifest := cloneRenderManifest(t, fixture.Manifest)
	manifest.Assets[0].ClipIDs = []string{"other-clip"}
	_, err := EvaluateVisualFrameStateForRenderManifest(manifest, 0)
	if err == nil || !strings.Contains(err.Error(), `source provenance asset "visual-source" does not bind clip "visual-clip"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateSourceProvenanceRejectsPartialMediaDimensions(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	manifest := cloneRenderManifest(t, fixture.Manifest)
	manifest.Assets[0].Media.Height = nil
	_, err := EvaluateSourceProvenance(manifest)
	if err == nil || !strings.Contains(err.Error(), `source provenance asset "visual-source" must provide both media width and height`) {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateSourceProvenanceRejectsNonPositiveMediaDimensions(t *testing.T) {
	fixture := loadSourceProvenanceFixture(t)
	manifest := cloneRenderManifest(t, fixture.Manifest)
	zero := 0
	manifest.Assets[0].Media.Width = &zero
	_, err := EvaluateSourceProvenance(manifest)
	if err == nil || !strings.Contains(err.Error(), `source provenance asset "visual-source" media width and height must be positive`) {
		t.Fatalf("error = %v", err)
	}
}

func cloneRenderManifest(t *testing.T, source RenderManifestV1) RenderManifestV1 {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var copy RenderManifestV1
	if err := json.Unmarshal(data, &copy); err != nil {
		t.Fatalf("clone manifest: %v", err)
	}
	return copy
}
