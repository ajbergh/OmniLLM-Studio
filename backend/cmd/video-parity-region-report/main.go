// Command video-parity-region-report measures tolerant browser↔renderer parity
// inside frame-indexed regions without changing structural-exactness semantics in
// video-parity-report. It is intended for codec/color diagnostics where decoded
// RGB values can differ slightly while canonical geometry remains identical.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

type regionManifest struct {
	Version int                              `json:"version"`
	Frames  []video.ParityFixtureRegionFrame `json:"frames"`
}

type regionMeasurement struct {
	FrameIndex int64                   `json:"frame_index"`
	Region     video.ParityRegion      `json:"region"`
	Metric     video.ParityFrameMetric `json:"metric"`
}

type report struct {
	SchemaVersion int                    `json:"schema_version"`
	Fixture       string                 `json:"fixture"`
	Thresholds    video.ParityThresholds `json:"thresholds"`
	Regions       []regionMeasurement    `json:"regions"`
	Pass          bool                   `json:"pass"`
}

func main() {
	previewDir := flag.String("preview-dir", "", "directory containing canonical preview PNGs")
	renderedDir := flag.String("rendered-dir", "", "directory containing renderer PNGs with matching names")
	regionsPath := flag.String("regions", "", "versioned JSON region manifest keyed by canonical frame index")
	outputPath := flag.String("output", "video-parity-region-report.json", "JSON output path")
	fixture := flag.String("fixture", "", "fixture name recorded in the report")
	fps := flag.Int("fps", 30, "timeline frames per second")
	allowFail := flag.Bool("allow-fail", false, "write diagnostic artifacts without returning exit code 2 when thresholds fail")
	flag.Parse()

	if strings.TrimSpace(*previewDir) == "" || strings.TrimSpace(*renderedDir) == "" || strings.TrimSpace(*regionsPath) == "" {
		exitf("--preview-dir, --rendered-dir, and --regions are required")
	}
	if *fps <= 0 {
		exitf("--fps must be positive")
	}
	manifest, err := loadManifest(*regionsPath)
	if err != nil {
		exitf("load region manifest: %v", err)
	}
	previewFrames, err := frameFiles(*previewDir)
	if err != nil {
		exitf("index preview frames: %v", err)
	}
	renderedFrames, err := frameFiles(*renderedDir)
	if err != nil {
		exitf("index rendered frames: %v", err)
	}

	thresholds := video.DefaultParityThresholds()
	result := report{SchemaVersion: 1, Fixture: *fixture, Thresholds: thresholds, Pass: true}
	for _, frame := range manifest.Frames {
		previewPath, previewOK := previewFrames[frame.FrameIndex]
		renderedPath, renderedOK := renderedFrames[frame.FrameIndex]
		if !previewOK || !renderedOK {
			exitf("frame_index %d is missing a preview/rendered PNG pair", frame.FrameIndex)
		}
		preview, err := decodeImage(previewPath)
		if err != nil {
			exitf("decode preview frame %d: %v", frame.FrameIndex, err)
		}
		rendered, err := decodeImage(renderedPath)
		if err != nil {
			exitf("decode rendered frame %d: %v", frame.FrameIndex, err)
		}
		for _, region := range frame.Regions {
			previewCrop, err := crop(preview, region.Bounds)
			if err != nil {
				exitf("crop preview frame %d region %q: %v", frame.FrameIndex, region.Name, err)
			}
			renderedCrop, err := crop(rendered, region.Bounds)
			if err != nil {
				exitf("crop rendered frame %d region %q: %v", frame.FrameIndex, region.Name, err)
			}
			metric := video.CompareParityFrame(video.ParityFramePair{
				Name:       region.Name,
				FrameIndex: frame.FrameIndex,
				TimeMS:     frame.FrameIndex * 1000 / int64(*fps),
				Preview:    previewCrop,
				Rendered:   renderedCrop,
			}, thresholds)
			result.Regions = append(result.Regions, regionMeasurement{FrameIndex: frame.FrameIndex, Region: region, Metric: metric})
			result.Pass = result.Pass && metric.Pass
		}
	}
	sort.SliceStable(result.Regions, func(i, j int) bool {
		if result.Regions[i].FrameIndex != result.Regions[j].FrameIndex {
			return result.Regions[i].FrameIndex < result.Regions[j].FrameIndex
		}
		return result.Regions[i].Region.Name < result.Regions[j].Region.Name
	})
	if err := writeJSON(*outputPath, result); err != nil {
		exitf("write report: %v", err)
	}
	fmt.Printf("video parity region report: %s (%d regions, pass=%t)\n", *outputPath, len(result.Regions), result.Pass)
	if !result.Pass && !*allowFail {
		os.Exit(2)
	}
}

func loadManifest(path string) (regionManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return regionManifest{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest regionManifest
	if err := decoder.Decode(&manifest); err != nil {
		return regionManifest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return regionManifest{}, fmt.Errorf("manifest must contain exactly one JSON object")
		}
		return regionManifest{}, err
	}
	if manifest.Version != 1 {
		return regionManifest{}, fmt.Errorf("manifest version = %d, want 1", manifest.Version)
	}
	return manifest, nil
}

func frameFiles(dir string) (map[int64]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".png" {
			continue
		}
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), "-", 2)
		frameIndex, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		if _, duplicate := result[frameIndex]; duplicate {
			return nil, fmt.Errorf("duplicate PNG for frame_index %d", frameIndex)
		}
		result[frameIndex] = filepath.Join(dir, entry.Name())
	}
	return result, nil
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}

func crop(src image.Image, bounds video.ParityBounds) (image.Image, error) {
	sourceBounds := src.Bounds()
	if bounds.MinX < 0 || bounds.MinY < 0 || bounds.MaxX <= bounds.MinX || bounds.MaxY <= bounds.MinY ||
		bounds.MaxX > sourceBounds.Dx() || bounds.MaxY > sourceBounds.Dy() {
		return nil, fmt.Errorf("bounds [%d,%d)-[%d,%d) exceed image %dx%d", bounds.MinX, bounds.MinY, bounds.MaxX, bounds.MaxY, sourceBounds.Dx(), sourceBounds.Dy())
	}
	width := bounds.MaxX - bounds.MinX
	height := bounds.MaxY - bounds.MinY
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	sourcePoint := image.Pt(sourceBounds.Min.X+bounds.MinX, sourceBounds.Min.Y+bounds.MinY)
	draw.Draw(dst, dst.Bounds(), src, sourcePoint, draw.Src)
	return dst, nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
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
