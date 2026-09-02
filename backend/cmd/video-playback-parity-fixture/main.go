// Command video-playback-parity-fixture emits the deterministic retained
// browser fixture for normal-playback canonicalization evidence.
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
	Cases    []video.PlaybackParityCase `json:"cases"`
}

func main() {
	outputDir := flag.String("output-dir", "video-renderer/test/fixtures/generated", "fixture output directory")
	flag.Parse()

	doc, assets, cases := video.PlaybackCanonicalParityFixture()
	validated, err := video.ValidateTimelineDocument(doc)
	if err != nil {
		exitf("validate playback fixture: %v", err)
	}
	bundle := fixtureBundle{
		Name:     video.PlaybackCanonicalParityFixtureName,
		Timeline: validated,
		Assets:   assets,
		Cases:    cases,
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		exitf("create output directory: %v", err)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		exitf("marshal playback fixture: %v", err)
	}
	data = append(data, '\n')
	path := filepath.Join(*outputDir, video.PlaybackCanonicalParityFixtureName+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		exitf("write playback fixture: %v", err)
	}
	fmt.Printf("video playback parity fixture: %s (%d cases)\n", path, len(cases))
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
