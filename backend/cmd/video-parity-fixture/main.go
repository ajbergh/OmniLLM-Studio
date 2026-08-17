// Command video-parity-fixture emits the deterministic Phase 0 timeline,
// asset recipe, named frame samples, and (optionally) FFmpeg source media.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

type fixtureBundle struct {
	Name     string                     `json:"name"`
	Seed     int64                      `json:"seed"`
	Timeline video.TimelineDocument     `json:"timeline"`
	Assets   []video.ParityFixtureAsset `json:"assets"`
	Samples  []video.ParityFrameSample  `json:"samples"`
}

func main() {
	outputDir := flag.String("output-dir", "video-renderer/test/fixtures/generated", "fixture output directory")
	seed := flag.Int64("seed", 20260817, "seed for deterministic random frame samples")
	randomSamples := flag.Int("random-samples", 8, "number of seeded random frame samples")
	generateMedia := flag.Bool("generate-media", false, "generate deterministic media assets with FFmpeg")
	flag.Parse()

	doc, assets := video.ParityTortureFixture()
	validated, err := video.ValidateTimelineDocument(doc)
	if err != nil {
		exitf("validate fixture: %v", err)
	}
	bundle := fixtureBundle{Name: video.ParityTortureFixtureName, Seed: *seed, Timeline: validated, Assets: assets, Samples: video.ParityFrameSamples(validated, *seed, *randomSamples)}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		exitf("create output directory: %v", err)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		exitf("marshal fixture: %v", err)
	}
	data = append(data, '\n')
	path := filepath.Join(*outputDir, "parity-torture-v1.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		exitf("write fixture: %v", err)
	}
	if *generateMedia {
		if err := generateFixtureMedia(*outputDir); err != nil {
			exitf("generate media: %v", err)
		}
	}
	fmt.Printf("video parity fixture: %s (%d samples)\n", path, len(bundle.Samples))
}

func generateFixtureMedia(outputDir string) error {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg was not found in PATH")
	}
	mediaDir := filepath.Join(outputDir, "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return err
	}
	commands := [][]string{
		{"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=30:duration=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=24", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", filepath.Join(mediaDir, "asset-landscape.mp4")},
		{"-f", "lavfi", "-i", "smptebars=size=360x640:rate=30:duration=24", "-f", "lavfi", "-i", "sine=frequency=660:sample_rate=48000:duration=24", "-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", filepath.Join(mediaDir, "asset-portrait.mp4")},
		{"-f", "lavfi", "-i", "testsrc2=size=512x512:rate=1:duration=1", "-frames:v", "1", filepath.Join(mediaDir, "asset-square.png")},
		// A swept tone plus deterministic one-second impulses makes sample-offset
		// correlation unambiguous; a constant sine aliases at many offsets and can
		// make a correct mix appear shifted by a whole number of periods.
		{"-f", "lavfi", "-i", "aevalsrc=0.09*sin(2*PI*(220*t+30*t*t))+if(lt(mod(t\\,1)\\,0.004)\\,0.35\\,0):s=48000:d=24", "-ac", "1", "-c:a", "pcm_s16le", filepath.Join(mediaDir, "asset-audio.wav")},
	}
	for _, args := range commands {
		full := append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)
		cmd := exec.Command(ffmpeg, full...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %w: %s", args, err, output)
		}
	}
	return nil
}

func exitf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
