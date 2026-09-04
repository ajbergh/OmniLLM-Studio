// Command video-cursor-smoothing-parity-fixture emits the focused
// parity-cursor-smoothing-v2 timeline used to compare canonical browser cursor
// smoothing with FFmpeg diagnostic frames.
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

func main() {
	outputDir := flag.String("output-dir", "output/video-cursor-smoothing-parity/fixture", "directory for generated fixture JSON")
	flag.Parse()

	doc, assets := video.ParityCursorSmoothingFixture()
	validated, err := video.ValidateTimelineDocument(doc)
	if err != nil {
		fatalf("validate cursor smoothing fixture: %v", err)
	}
	bundle := fixtureBundle{
		Name:     video.ParityCursorSmoothingFixtureName,
		Timeline: validated,
		Assets:   assets,
		Samples:  video.ParityCursorSmoothingFrameSamples(),
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}
	body, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fatalf("marshal fixture: %v", err)
	}
	body = append(body, '\n')
	path := filepath.Join(*outputDir, video.ParityCursorSmoothingFixtureName+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fatalf("write fixture: %v", err)
	}
	fmt.Println(path)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
