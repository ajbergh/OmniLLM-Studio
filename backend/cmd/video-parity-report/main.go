// Command video-parity-report compares canonical preview PNGs with decoded
// renderer PNGs and emits JSON, Markdown, side-by-side, and heat-map artifacts.
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

func main() {
	previewDir := flag.String("preview-dir", "", "directory containing canonical preview PNGs")
	renderedDir := flag.String("rendered-dir", "", "directory containing renderer PNGs with matching names")
	outputDir := flag.String("output-dir", "video-parity-report", "artifact output directory")
	fixture := flag.String("fixture", "parity-torture-v1", "fixture name recorded in the report")
	fps := flag.Int("fps", 30, "timeline frames per second")
	timelineHash := flag.String("timeline-sha256", "", "immutable timeline hash")
	manifestHash := flag.String("manifest-sha256", "", "immutable asset manifest hash")
	regionsPath := flag.String("regions", "", "optional versioned JSON structural-region manifest keyed by canonical frame index")
	previewAudio := flag.String("preview-audio", "", "optional signed 16-bit little-endian PCM (interleaved channels are compared as a sample sequence)")
	renderedAudio := flag.String("rendered-audio", "", "optional signed 16-bit little-endian PCM (interleaved channels are compared as a sample sequence)")
	maxAudioOffset := flag.Int("max-audio-offset", 2400, "maximum offset searched when aligning PCM samples")
	ffmpeg := flag.String("ffmpeg", "", "optional FFmpeg binary path used for EBU R128 audio measurement")
	deliveryMedia := flag.String("delivery-media", "", "optional final delivery video to validate with FFprobe")
	expectedDurationMS := flag.Int64("expected-duration-ms", 0, "expected delivery timeline duration in milliseconds")
	ffprobe := flag.String("ffprobe", "", "optional FFprobe binary path")
	allowFail := flag.Bool("allow-fail", false, "write baseline artifacts without returning exit code 2 when parity thresholds fail")
	flag.Parse()

	if strings.TrimSpace(*previewDir) == "" || strings.TrimSpace(*renderedDir) == "" {
		exitf("--preview-dir and --rendered-dir are required")
	}
	if *fps <= 0 {
		exitf("--fps must be positive")
	}
	regionsByFrame, err := loadParityRegionManifest(*regionsPath)
	if err != nil {
		exitf("load parity region manifest: %v", err)
	}
	pairs, err := loadPairs(*previewDir, *renderedDir, *fps, regionsByFrame)
	if err != nil {
		exitf("load frame pairs: %v", err)
	}
	if len(pairs) == 0 {
		exitf("no matching PNG frame names were found")
	}

	thresholds := video.DefaultParityThresholds()
	var audioMetric *video.ParityAudioMetric
	if *previewAudio != "" || *renderedAudio != "" {
		if *previewAudio == "" || *renderedAudio == "" {
			exitf("both audio paths are required when comparing audio")
		}
		preview, err := readPCM16(*previewAudio)
		if err != nil {
			exitf("read preview audio: %v", err)
		}
		rendered, err := readPCM16(*renderedAudio)
		if err != nil {
			exitf("read rendered audio: %v", err)
		}
		metric := video.CompareParityAudio(preview, rendered, *maxAudioOffset, thresholds)
		previewLoudness, err := video.MeasureParityEBUR128(context.Background(), *ffmpeg, *previewAudio)
		if err != nil {
			exitf("measure preview audio loudness: %v", err)
		}
		renderedLoudness, err := video.MeasureParityEBUR128(context.Background(), *ffmpeg, *renderedAudio)
		if err != nil {
			exitf("measure rendered audio loudness: %v", err)
		}
		video.ApplyParityEBUR128(&metric, previewLoudness, renderedLoudness, thresholds)
		audioMetric = &metric
	}
	report := video.BuildParityReport(*fixture, *timelineHash, *manifestHash, pairs, audioMetric, thresholds)
	if strings.TrimSpace(*deliveryMedia) != "" {
		metric, err := video.ProbeParityDelivery(context.Background(), *ffprobe, *deliveryMedia, *expectedDurationMS, float64(*fps))
		if err != nil {
			exitf("probe delivery: %v", err)
		}
		report.Delivery = &metric
		report.Pass = report.Pass && metric.Pass
	}
	if err := video.WriteParityReport(*outputDir, report, pairs); err != nil {
		exitf("write report: %v", err)
	}
	fmt.Printf("video parity report: %s (%d frames, pass=%t)\n", filepath.Join(*outputDir, "parity-report.json"), len(pairs), report.Pass)
	if !report.Pass && !*allowFail {
		os.Exit(2)
	}
}

func loadPairs(previewDir, renderedDir string, fps int, regionsByFrame map[int64][]video.ParityRegion) ([]video.ParityFramePair, error) {
	entries, err := os.ReadDir(previewDir)
	if err != nil {
		return nil, err
	}
	var pairs []video.ParityFramePair
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".png" {
			continue
		}
		renderedPath := filepath.Join(renderedDir, entry.Name())
		if _, err := os.Stat(renderedPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		preview, err := decodeImage(filepath.Join(previewDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		rendered, err := decodeImage(renderedPath)
		if err != nil {
			return nil, err
		}
		index, name := parseFrameName(entry.Name())
		regions := cloneParityRegions(regionsByFrame[index])
		if err := validateParityRegionsForPair(index, regions, preview, rendered); err != nil {
			return nil, err
		}
		pairs = append(pairs, video.ParityFramePair{
			Name:       name,
			FrameIndex: index,
			TimeMS:     index * 1000 / int64(fps),
			Preview:    preview,
			Rendered:   rendered,
			Regions:    regions,
		})
	}
	video.SortParityPairs(pairs)
	return pairs, nil
}

func parseFrameName(fileName string) (int64, string) {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	parts := strings.SplitN(base, "-", 2)
	index, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, base
	}
	if len(parts) == 2 && parts[1] != "" {
		return index, parts[1]
	}
	return index, base
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
func readPCM16(path string) ([]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("PCM byte count must be even")
	}
	samples := make([]float64, len(data)/2)
	for i := range samples {
		samples[i] = float64(int16(binary.LittleEndian.Uint16(data[i*2:]))) / 32768
	}
	return samples, nil
}
func exitf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
