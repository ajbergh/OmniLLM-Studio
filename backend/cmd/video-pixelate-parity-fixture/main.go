// Command video-pixelate-parity-fixture emits the focused opaque pixelate
// timeline, named frame samples, and frame-indexed structural-region policy.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

type fixtureBundle struct {
	Name     string                     `json:"name"`
	Timeline video.TimelineDocument     `json:"timeline"`
	Assets   []video.ParityFixtureAsset `json:"assets"`
	Samples  []video.ParityFrameSample  `json:"samples"`
}

type regionManifest struct {
	Version int                              `json:"version"`
	Frames  []video.ParityFixtureRegionFrame `json:"frames"`
}

func main() {
	outputDir := "video-renderer/test/fixtures/generated"
	if len(os.Args) == 3 && os.Args[1] == "--output-dir" {
		outputDir = os.Args[2]
	} else if len(os.Args) != 1 {
		exitf("usage: video-pixelate-parity-fixture [--output-dir <dir>]")
	}

	doc, assets := video.ParityPixelateOpaqueFixture()
	validated, err := video.ValidateTimelineDocument(doc)
	if err != nil {
		exitf("validate fixture: %v", err)
	}
	samples := video.ParityPixelateOpaqueFrameSamples()
	bundle := fixtureBundle{
		Name:     video.ParityPixelateOpaqueFixtureName,
		Timeline: validated,
		Assets:   assets,
		Samples:  samples,
	}
	regions := regionManifest{
		Version: 1,
		Frames:  video.ParityPixelateOpaqueRegionFrames(samples),
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		exitf("create output directory: %v", err)
	}
	fixturePath := filepath.Join(outputDir, video.ParityPixelateOpaqueFixtureName+".json")
	if err := writeJSON(fixturePath, bundle); err != nil {
		exitf("write fixture: %v", err)
	}
	regionsPath := filepath.Join(outputDir, video.ParityPixelateOpaqueFixtureName+".regions.json")
	if err := writeJSON(regionsPath, regions); err != nil {
		exitf("write region manifest: %v", err)
	}
	fmt.Printf("video pixelate parity fixture: %s (%d samples)\n", fixturePath, len(samples))
	fmt.Printf("video pixelate parity regions: %s\n", regionsPath)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
