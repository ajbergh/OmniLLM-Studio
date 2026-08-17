package video

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultParityChannelTolerance = uint8(2)
	DefaultParityPixelPassRate    = 0.999
	DefaultParitySSIM             = 0.995
	DefaultParityAudioCorrelation = 0.999
	DefaultParityAudioOffset      = 1
)

type ParityBounds struct {
	MinX int `json:"min_x"`
	MinY int `json:"min_y"`
	MaxX int `json:"max_x"`
	MaxY int `json:"max_y"`
}

type ParityRegion struct {
	Name   string       `json:"name"`
	Bounds ParityBounds `json:"bounds"`
}

type ParityRegionMetric struct {
	Name           string       `json:"name"`
	Bounds         ParityBounds `json:"bounds"`
	ComparedPixels int64        `json:"compared_pixels"`
	ChangedPixels  int64        `json:"changed_pixels"`
	Exact          bool         `json:"exact"`
}

type ParityFrameMetric struct {
	Name                  string               `json:"name"`
	FrameIndex            int64                `json:"frame_index"`
	TimeMS                int64                `json:"time_ms"`
	PreviewWidth          int                  `json:"preview_width"`
	PreviewHeight         int                  `json:"preview_height"`
	RenderedWidth         int                  `json:"rendered_width"`
	RenderedHeight        int                  `json:"rendered_height"`
	DimensionsMatch       bool                 `json:"dimensions_match"`
	ComparedPixels        int64                `json:"compared_pixels"`
	PixelsWithinTolerance int64                `json:"pixels_within_tolerance"`
	PixelPassRate         float64              `json:"pixel_pass_rate"`
	MeanAbsoluteError     float64              `json:"mean_absolute_error"`
	RootMeanSquareError   float64              `json:"root_mean_square_error"`
	MaxChannelDelta       uint8                `json:"max_channel_delta"`
	SSIM                  float64              `json:"ssim"`
	ChangedBounds         *ParityBounds        `json:"changed_bounds,omitempty"`
	StructuralRegions     []ParityRegionMetric `json:"structural_regions,omitempty"`
	Pass                  bool                 `json:"pass"`
}

type ParityAudioMetric struct {
	PreviewSamples     int     `json:"preview_samples"`
	RenderedSamples    int     `json:"rendered_samples"`
	OffsetSamples      int     `json:"offset_samples"`
	Correlation        float64 `json:"correlation"`
	PreviewPeak        float64 `json:"preview_peak"`
	RenderedPeak       float64 `json:"rendered_peak"`
	PeakDifference     float64 `json:"peak_difference"`
	PreviewLUFSApprox  float64 `json:"preview_lufs_approx"`
	RenderedLUFSApprox float64 `json:"rendered_lufs_approx"`
	LUFSDifference     float64 `json:"lufs_difference"`
	Pass               bool    `json:"pass"`
}

type ParityThresholds struct {
	ChannelTolerance        uint8   `json:"channel_tolerance"`
	MinimumPixelPassRate    float64 `json:"minimum_pixel_pass_rate"`
	MinimumSSIM             float64 `json:"minimum_ssim"`
	MinimumAudioCorrelation float64 `json:"minimum_audio_correlation"`
	MaximumAudioOffset      int     `json:"maximum_audio_offset_samples"`
}

func DefaultParityThresholds() ParityThresholds {
	return ParityThresholds{
		ChannelTolerance:        DefaultParityChannelTolerance,
		MinimumPixelPassRate:    DefaultParityPixelPassRate,
		MinimumSSIM:             DefaultParitySSIM,
		MinimumAudioCorrelation: DefaultParityAudioCorrelation,
		MaximumAudioOffset:      DefaultParityAudioOffset,
	}
}

type ParityReport struct {
	SchemaVersion  int                 `json:"schema_version"`
	GeneratedAt    time.Time           `json:"generated_at"`
	Fixture        string              `json:"fixture"`
	TimelineSHA256 string              `json:"timeline_sha256,omitempty"`
	ManifestSHA256 string              `json:"asset_manifest_sha256,omitempty"`
	Thresholds     ParityThresholds    `json:"thresholds"`
	Frames         []ParityFrameMetric `json:"frames"`
	Audio          *ParityAudioMetric  `json:"audio,omitempty"`
	Pass           bool                `json:"pass"`
}

type ParityFramePair struct {
	Name       string
	FrameIndex int64
	TimeMS     int64
	Preview    image.Image
	Rendered   image.Image
	Regions    []ParityRegion
}

func CompareParityFrame(pair ParityFramePair, thresholds ParityThresholds) ParityFrameMetric {
	pb, rb := pair.Preview.Bounds(), pair.Rendered.Bounds()
	w, h := minInt(pb.Dx(), rb.Dx()), minInt(pb.Dy(), rb.Dy())
	metric := ParityFrameMetric{
		Name: pair.Name, FrameIndex: pair.FrameIndex, TimeMS: pair.TimeMS,
		PreviewWidth: pb.Dx(), PreviewHeight: pb.Dy(), RenderedWidth: rb.Dx(), RenderedHeight: rb.Dy(),
		DimensionsMatch: pb.Dx() == rb.Dx() && pb.Dy() == rb.Dy(),
	}
	if w <= 0 || h <= 0 {
		return metric
	}
	var absoluteSum, squaredSum float64
	var within, pixels int64
	var maxDelta uint8
	minX, minY, maxX, maxY := w, h, -1, -1
	previewLuma, renderedLuma := make([]float64, 0, w*h), make([]float64, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pr, pg, pbv, _ := pair.Preview.At(pb.Min.X+x, pb.Min.Y+y).RGBA()
			rr, rg, rbv, _ := pair.Rendered.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			pc := [3]uint8{uint8(pr >> 8), uint8(pg >> 8), uint8(pbv >> 8)}
			rc := [3]uint8{uint8(rr >> 8), uint8(rg >> 8), uint8(rbv >> 8)}
			pixelWithin, pixelChanged := true, false
			for channel := 0; channel < 3; channel++ {
				delta := absByteDelta(pc[channel], rc[channel])
				absoluteSum += float64(delta)
				squaredSum += float64(delta) * float64(delta)
				if delta > thresholds.ChannelTolerance {
					pixelWithin = false
				}
				if delta > 0 {
					pixelChanged = true
				}
				if delta > maxDelta {
					maxDelta = delta
				}
			}
			if pixelWithin {
				within++
			}
			if pixelChanged {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
			previewLuma = append(previewLuma, .2126*float64(pc[0])+.7152*float64(pc[1])+.0722*float64(pc[2]))
			renderedLuma = append(renderedLuma, .2126*float64(rc[0])+.7152*float64(rc[1])+.0722*float64(rc[2]))
			pixels++
		}
	}
	metric.ComparedPixels = pixels
	metric.PixelsWithinTolerance = within
	metric.PixelPassRate = float64(within) / float64(pixels)
	metric.MeanAbsoluteError = absoluteSum / float64(pixels*3)
	metric.RootMeanSquareError = math.Sqrt(squaredSum / float64(pixels*3))
	metric.MaxChannelDelta = maxDelta
	metric.SSIM = globalSSIM(previewLuma, renderedLuma)
	if maxX >= 0 {
		metric.ChangedBounds = &ParityBounds{MinX: minX, MinY: minY, MaxX: maxX + 1, MaxY: maxY + 1}
	}
	for _, region := range pair.Regions {
		metric.StructuralRegions = append(metric.StructuralRegions, compareRegion(pair.Preview, pair.Rendered, region))
	}
	structuralExact := true
	for _, region := range metric.StructuralRegions {
		structuralExact = structuralExact && region.Exact
	}
	metric.Pass = metric.DimensionsMatch && metric.PixelPassRate >= thresholds.MinimumPixelPassRate && metric.SSIM >= thresholds.MinimumSSIM && structuralExact
	return metric
}

func CompareParityAudio(preview, rendered []float64, maxOffset int, thresholds ParityThresholds) ParityAudioMetric {
	metric := ParityAudioMetric{PreviewSamples: len(preview), RenderedSamples: len(rendered)}
	bestCorrelation := -2.0
	for offset := -maxOffset; offset <= maxOffset; offset++ {
		correlation := correlationAtOffset(preview, rendered, offset)
		if correlation > bestCorrelation {
			bestCorrelation, metric.OffsetSamples = correlation, offset
		}
	}
	metric.Correlation = bestCorrelation
	metric.PreviewPeak, metric.RenderedPeak = signalPeak(preview), signalPeak(rendered)
	metric.PeakDifference = math.Abs(metric.PreviewPeak - metric.RenderedPeak)
	metric.PreviewLUFSApprox, metric.RenderedLUFSApprox = approximateLUFS(preview), approximateLUFS(rendered)
	metric.LUFSDifference = math.Abs(metric.PreviewLUFSApprox - metric.RenderedLUFSApprox)
	metric.Pass = math.Abs(float64(metric.OffsetSamples)) <= float64(thresholds.MaximumAudioOffset) && metric.Correlation >= thresholds.MinimumAudioCorrelation
	return metric
}

func BuildParityReport(fixture, timelineHash, manifestHash string, pairs []ParityFramePair, audio *ParityAudioMetric, thresholds ParityThresholds) ParityReport {
	report := ParityReport{SchemaVersion: 1, GeneratedAt: time.Now().UTC(), Fixture: fixture, TimelineSHA256: timelineHash, ManifestSHA256: manifestHash, Thresholds: thresholds, Audio: audio, Pass: true}
	for _, pair := range pairs {
		metric := CompareParityFrame(pair, thresholds)
		report.Frames = append(report.Frames, metric)
		report.Pass = report.Pass && metric.Pass
	}
	if audio != nil {
		report.Pass = report.Pass && audio.Pass
	}
	return report
}

func WriteParityReport(outputDir string, report ParityReport, pairs []ParityFramePair) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	framesDir := filepath.Join(outputDir, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return err
	}
	for _, pair := range pairs {
		name := sanitizePathSegment(pair.Name)
		if name == "" {
			name = fmt.Sprintf("frame-%d", pair.FrameIndex)
		}
		if len(name) > 96 {
			name = name[:80] + "-" + contentSHA256([]byte(name))[:12]
		}
		if err := writePNG(filepath.Join(framesDir, name+"-side-by-side.png"), sideBySideImage(pair.Preview, pair.Rendered)); err != nil {
			return err
		}
		if err := writePNG(filepath.Join(framesDir, name+"-heatmap.png"), heatmapImage(pair.Preview, pair.Rendered)); err != nil {
			return err
		}
	}
	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := os.WriteFile(filepath.Join(outputDir, "parity-report.json"), jsonBytes, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "parity-report.md"), []byte(parityReportMarkdown(report)), 0o644)
}

func parityReportMarkdown(report ParityReport) string {
	status := "FAIL"
	if report.Pass {
		status = "PASS"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Video render parity report\n\n- Fixture: `%s`\n- Status: **%s**\n- Generated: %s\n\n", report.Fixture, status, report.GeneratedAt.Format(time.RFC3339))
	b.WriteString("| Sample | Frame | Time (ms) | Pixels within tolerance | SSIM | Result |\n|---|---:|---:|---:|---:|---|\n")
	for _, frame := range report.Frames {
		frameStatus := "FAIL"
		if frame.Pass {
			frameStatus = "PASS"
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %.5f | %.6f | %s |\n", frame.Name, frame.FrameIndex, frame.TimeMS, frame.PixelPassRate, frame.SSIM, frameStatus)
	}
	if report.Audio != nil {
		fmt.Fprintf(&b, "\nAudio: offset `%d` samples, correlation `%.6f`, peak difference `%.6f`, approximate LUFS difference `%.3f`.\n", report.Audio.OffsetSamples, report.Audio.Correlation, report.Audio.PeakDifference, report.Audio.LUFSDifference)
	}
	return b.String()
}

func compareRegion(preview, rendered image.Image, region ParityRegion) ParityRegionMetric {
	pb, rb := preview.Bounds(), rendered.Bounds()
	minX, minY := maxInt(region.Bounds.MinX, 0), maxInt(region.Bounds.MinY, 0)
	maxX, maxY := minInt(region.Bounds.MaxX, minInt(pb.Dx(), rb.Dx())), minInt(region.Bounds.MaxY, minInt(pb.Dy(), rb.Dy()))
	metric := ParityRegionMetric{Name: region.Name, Bounds: region.Bounds, Exact: true}
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			metric.ComparedPixels++
			pr, pg, pbv, pa := preview.At(pb.Min.X+x, pb.Min.Y+y).RGBA()
			rr, rg, rbv, ra := rendered.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			if pr != rr || pg != rg || pbv != rbv || pa != ra {
				metric.ChangedPixels++
				metric.Exact = false
			}
		}
	}
	return metric
}

func globalSSIM(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var ma, mb float64
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(len(a))
	mb /= float64(len(b))
	var va, vb, cov float64
	for i := range a {
		da, db := a[i]-ma, b[i]-mb
		va += da * da
		vb += db * db
		cov += da * db
	}
	denom := float64(maxInt(len(a)-1, 1))
	va /= denom
	vb /= denom
	cov /= denom
	c1, c2 := math.Pow(.01*255, 2), math.Pow(.03*255, 2)
	return ((2*ma*mb + c1) * (2*cov + c2)) / ((ma*ma + mb*mb + c1) * (va + vb + c2))
}

func correlationAtOffset(a, b []float64, offset int) float64 {
	aStart, bStart := 0, 0
	if offset > 0 {
		bStart = offset
	} else {
		aStart = -offset
	}
	n := minInt(len(a)-aStart, len(b)-bStart)
	if n < 2 {
		return -1
	}
	var ma, mb float64
	for i := 0; i < n; i++ {
		ma += a[aStart+i]
		mb += b[bStart+i]
	}
	ma /= float64(n)
	mb /= float64(n)
	var va, vb, cov float64
	for i := 0; i < n; i++ {
		da, db := a[aStart+i]-ma, b[bStart+i]-mb
		va += da * da
		vb += db * db
		cov += da * db
	}
	if va == 0 || vb == 0 {
		if va == vb {
			return 1
		}
		return 0
	}
	return cov / math.Sqrt(va*vb)
}

func signalPeak(samples []float64) float64 {
	var peak float64
	for _, sample := range samples {
		if value := math.Abs(sample); value > peak {
			peak = value
		}
	}
	return peak
}
func approximateLUFS(samples []float64) float64 {
	if len(samples) == 0 {
		return -120
	}
	var power float64
	for _, sample := range samples {
		power += sample * sample
	}
	power /= float64(len(samples))
	if power <= 0 {
		return -120
	}
	return -.691 + 10*math.Log10(power)
}
func absByteDelta(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
func sideBySideImage(left, right image.Image) image.Image {
	lb, rb := left.Bounds(), right.Bounds()
	height := maxInt(lb.Dy(), rb.Dy())
	out := image.NewRGBA(image.Rect(0, 0, lb.Dx()+rb.Dx(), height))
	drawImage(out, left, 0)
	drawImage(out, right, lb.Dx())
	return out
}
func heatmapImage(left, right image.Image) image.Image {
	lb, rb := left.Bounds(), right.Bounds()
	w, h := minInt(lb.Dx(), rb.Dx()), minInt(lb.Dy(), rb.Dy())
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			lr, lg, lbv, _ := left.At(lb.Min.X+x, lb.Min.Y+y).RGBA()
			rr, rg, rbv, _ := right.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			d := maxInt(int(absByteDelta(uint8(lr>>8), uint8(rr>>8))), maxInt(int(absByteDelta(uint8(lg>>8), uint8(rg>>8))), int(absByteDelta(uint8(lbv>>8), uint8(rbv>>8)))))
			out.SetRGBA(x, y, color.RGBA{R: uint8(d), G: uint8(minInt(d*2, 255)), A: 255})
		}
	}
	return out
}
func drawImage(dst *image.RGBA, src image.Image, offsetX int) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(offsetX+x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
}
func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func SortParityPairs(pairs []ParityFramePair) {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].FrameIndex == pairs[j].FrameIndex {
			return pairs[i].Name < pairs[j].Name
		}
		return pairs[i].FrameIndex < pairs[j].FrameIndex
	})
}
