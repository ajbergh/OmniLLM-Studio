// Command video-pixelate-parity-fixture emits focused pixelate timelines,
// named frame samples, and frame-indexed region policy for retained evidence.
package main

import (
	"encoding/json"
	"flag"
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
	outputDir := flag.String("output-dir", "video-renderer/test/fixtures/generated", "fixture output directory")
	variant := flag.String("variant", "opaque-png", "fixture variant: opaque-png, decoded-video, alpha-png, or alpha-video")
	flag.Parse()

	var (
		name    string
		doc     video.TimelineDocument
		assets  []video.ParityFixtureAsset
		samples []video.ParityFrameSample
		frames  []video.ParityFixtureRegionFrame
	)
	switch *variant {
	case "opaque-png":
		name = video.ParityPixelateOpaqueFixtureName
		doc, assets = video.ParityPixelateOpaqueFixture()
		samples = video.ParityPixelateOpaqueFrameSamples()
		frames = video.ParityPixelateOpaqueRegionFrames(samples)
	case "decoded-video":
		name = video.ParityPixelateDecodedVideoFixtureName
		doc, assets = video.ParityPixelateDecodedVideoFixture()
		samples = video.ParityPixelateDecodedVideoFrameSamples()
		frames = video.ParityPixelateDecodedVideoRegionFrames(samples)
	case "alpha-png":
		name = video.ParityPixelateAlphaFixtureName
		doc, assets = video.ParityPixelateAlphaFixture()
		samples = video.ParityPixelateAlphaFrameSamples()
		frames = video.ParityPixelateAlphaRegionFrames(samples)
	case "alpha-video":
		name = video.ParityPixelateAlphaVideoFixtureName
		doc, assets = video.ParityPixelateAlphaVideoFixture()
		samples = video.ParityPixelateAlphaVideoFrameSamples()
		frames = video.ParityPixelateAlphaVideoRegionFrames(samples)
	default:
		exitf("unknown --variant %q (want opaque-png, decoded-video, alpha-png, or alpha-video)", *variant)
	}

	validated, err := video.ValidateTimelineDocument(doc)
	if err != nil {
		exitf("validate fixture: %v", err)
	}
	bundle := fixtureBundle{
		Name:     name,
		Timeline: validated,
		Assets:   assets,
		Samples:  samples,
	}
	regions := regionManifest{Version: 1, Frames: frames}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		exitf("create output directory: %v", err)
	}
	fixturePath := filepath.Join(*outputDir, name+".json")
	if err := writeJSON(fixturePath, bundle); err != nil {
		exitf("write fixture: %v", err)
	}
	regionsPath := filepath.Join(*outputDir, name+".regions.json")
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
