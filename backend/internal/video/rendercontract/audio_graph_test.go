package rendercontract

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type audioGraphFixture struct {
	Manifest RenderManifestV1 `json:"manifest"`
	Expected AudioGraphV1      `json:"expected"`
}

func loadAudioGraphFixture(t *testing.T) audioGraphFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/audio_graph_v1.json")
	if err != nil {
		t.Fatalf("read audio graph fixture: %v", err)
	}
	var fixture audioGraphFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode audio graph fixture: %v", err)
	}
	return fixture
}

func TestEvaluateAudioGraphV1MatchesSharedFixture(t *testing.T) {
	fixture := loadAudioGraphFixture(t)
	graph, err := EvaluateAudioGraphV1(fixture.Manifest)
	if err != nil {
		t.Fatalf("EvaluateAudioGraphV1: %v", err)
	}
	if !reflect.DeepEqual(graph, fixture.Expected) {
		got, _ := json.MarshalIndent(graph, "", "  ")
		want, _ := json.MarshalIndent(fixture.Expected, "", "  ")
		t.Fatalf("audio graph mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestEvaluateAudioGraphV1FailsClosedOnUnsupportedChannelMapping(t *testing.T) {
	fixture := loadAudioGraphFixture(t)
	channels := 6
	fixture.Manifest.Assets[0].Media.Channels = &channels
	_, err := EvaluateAudioGraphV1(fixture.Manifest)
	if err == nil || !strings.Contains(err.Error(), "no canonical v1 channel mapping") {
		t.Fatalf("expected unsupported channel mapping error, got %v", err)
	}
}

func TestEvaluateAudioGraphV1FailsClosedOnUnknownProgramProcessingField(t *testing.T) {
	fixture := loadAudioGraphFixture(t)
	processing := fixture.Manifest.Timeline.Metadata["render_audio_processing"].(map[string]any)
	processing["custom_curve"] = []any{0.0, 1.0}
	_, err := EvaluateAudioGraphV1(fixture.Manifest)
	if err == nil || !strings.Contains(err.Error(), "unsupported field") {
		t.Fatalf("expected unsupported processing field error, got %v", err)
	}
}

func TestEvaluateAudioGraphV1KeepsMutedAndSoloSuppressedSourcesAddressable(t *testing.T) {
	fixture := loadAudioGraphFixture(t)
	graph, err := EvaluateAudioGraphV1(fixture.Manifest)
	if err != nil {
		t.Fatalf("EvaluateAudioGraphV1: %v", err)
	}
	reasons := map[string]string{}
	for _, source := range graph.Sources {
		reasons[source.ClipID] = source.SuppressionReason
	}
	if reasons["clip-muted"] != "clip-muted" {
		t.Fatalf("clip-muted suppression=%q", reasons["clip-muted"])
	}
	if reasons["clip-b"] != "solo-suppressed" {
		t.Fatalf("clip-b suppression=%q", reasons["clip-b"])
	}
	if reasons["clip-c"] != "track-muted" {
		t.Fatalf("clip-c suppression=%q", reasons["clip-c"])
	}
}

func TestAudioGraphSampleBoundaryMathIsExactForNonIntegralFrameSamples(t *testing.T) {
	if got := frameToSamplesFloor(1, 29, 48000); got != 1655 {
		t.Fatalf("frameToSamplesFloor=%d want 1655", got)
	}
	if got := frameToSamplesCeil(1, 29, 48000); got != 1656 {
		t.Fatalf("frameToSamplesCeil=%d want 1656", got)
	}
}
