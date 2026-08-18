package video

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestParityTortureFixtureValidatesAndSamplesDeterministically(t *testing.T) {
	doc, assets := ParityTortureFixture()
	validated, err := ValidateTimelineDocument(doc)
	if err != nil {
		t.Fatalf("validate parity fixture: %v", err)
	}
	if len(assets) != 4 || len(validated.Tracks) < 9 || len(validated.Scenes) != 2 {
		t.Fatalf("fixture coverage changed unexpectedly: assets=%d tracks=%d scenes=%d", len(assets), len(validated.Tracks), len(validated.Scenes))
	}
	first := ParityFrameSamples(validated, 20260817, 8)
	second := ParityFrameSamples(validated, 20260817, 8)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("parity frame sampling is not deterministic")
	}
	if len(first) < 30 {
		t.Fatalf("expected boundary/keyframe/random coverage, got %d samples", len(first))
	}
	seenFrames := map[int64]bool{}
	reasons := map[string]bool{}
	for _, sample := range first {
		if seenFrames[sample.FrameIndex] {
			t.Fatalf("duplicate frame sample %d", sample.FrameIndex)
		}
		seenFrames[sample.FrameIndex] = true
		for _, reason := range strings.Split(sample.Reason, "; ") {
			reasons[reason] = true
		}
	}
	for _, reason := range []string{"timeline boundary", "clip start", "keyframe", "transition midpoint", "scene boundary", "seeded random"} {
		if !reasons[reason] {
			t.Errorf("missing %q sample", reason)
		}
	}
}

func TestParityFrameSamplesUseCanonicalHighFPSMapping(t *testing.T) {
	doc := NewEmptyTimeline(160, 90, 120)
	doc.DurationMS = 10
	doc.Tracks = []TimelineTrack{{ID: "track", Type: TrackTypeLayer, Name: "Track", Visible: true, Clips: []TimelineClip{{
		ID: "subframe", StartMS: 5, DurationMS: 5, TrimOutMS: 5, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{},
	}}}}
	samples := ParityFrameSamples(doc, 1, 0)
	if len(samples) != 2 {
		t.Fatalf("high-FPS samples = %+v", samples)
	}
	if samples[0].FrameIndex != 0 || !strings.Contains(samples[0].Name, "clip-start-subframe") {
		t.Fatalf("sub-frame clip start did not map to frame zero: %+v", samples)
	}
	if samples[1].FrameIndex != 1 {
		t.Fatalf("last timeline sample frame = %d, want 1", samples[1].FrameIndex)
	}
}

func TestParityMetricsAndArtifacts(t *testing.T) {
	preview := image.NewRGBA(image.Rect(0, 0, 4, 3))
	rendered := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			pixel := color.RGBA{R: uint8(30 + x), G: uint8(60 + y), B: 90, A: 255}
			preview.SetRGBA(x, y, pixel)
			rendered.SetRGBA(x, y, pixel)
		}
	}
	thresholds := DefaultParityThresholds()
	pair := ParityFramePair{Name: "exact", FrameIndex: 12, TimeMS: 400, Preview: preview, Rendered: rendered, Regions: []ParityRegion{{Name: "text-bounds", Bounds: ParityBounds{MinX: 0, MinY: 0, MaxX: 2, MaxY: 2}}}}
	metric := CompareParityFrame(pair, thresholds)
	if !metric.Pass || metric.PixelPassRate != 1 || metric.SSIM < .999999 || metric.ChangedBounds != nil {
		t.Fatalf("identical frame did not pass: %+v", metric)
	}
	rendered.SetRGBA(1, 1, color.RGBA{R: 255, A: 255})
	metric = CompareParityFrame(pair, thresholds)
	if metric.Pass || metric.ChangedBounds == nil || metric.StructuralRegions[0].Exact {
		t.Fatalf("structural frame difference was not detected: %+v", metric)
	}

	signal := []float64{0, .2, .7, -.4, .1, 0}
	shifted := append([]float64{0, 0}, signal...)
	audio := CompareParityAudio(signal, shifted, 4, thresholds)
	if audio.OffsetSamples != 2 || audio.Correlation < .999999 {
		t.Fatalf("audio alignment mismatch: %+v", audio)
	}
	if audio.SampleCountsMatch || audio.Pass {
		t.Fatalf("different sample counts must fail parity: %+v", audio)
	}

	report := BuildParityReport("test-fixture", "timeline-hash", "manifest-hash", []ParityFramePair{pair}, &audio, thresholds)
	output := t.TempDir()
	if err := WriteParityReport(output, report, []ParityFramePair{pair}); err != nil {
		t.Fatalf("write parity artifacts: %v", err)
	}
	for _, name := range []string{"parity-report.json", "parity-report.md", filepath.Join("frames", "000012-exact-side-by-side.png"), filepath.Join("frames", "000012-exact-heatmap.png")} {
		if info, err := os.Stat(filepath.Join(output, name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing parity artifact %s: %v", name, err)
		}
	}
}

func TestParityDiagnosticArtifactNamesIncludeFrameIndex(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	pairs := []ParityFramePair{
		{Name: "repeated-sample", FrameIndex: 21, TimeMS: 700, Preview: frame, Rendered: frame},
		{Name: "repeated-sample", FrameIndex: 141, TimeMS: 4700, Preview: frame, Rendered: frame},
	}
	report := BuildParityReport("test-fixture", "timeline-hash", "manifest-hash", pairs, nil, DefaultParityThresholds())
	output := t.TempDir()
	if err := WriteParityReport(output, report, pairs); err != nil {
		t.Fatalf("write parity artifacts: %v", err)
	}
	for _, name := range []string{
		"000021-repeated-sample-side-by-side.png",
		"000021-repeated-sample-heatmap.png",
		"000141-repeated-sample-side-by-side.png",
		"000141-repeated-sample-heatmap.png",
	} {
		if info, err := os.Stat(filepath.Join(output, "frames", name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing unique parity artifact %s: %v", name, err)
		}
	}
}

func TestParseAndApplyParityEBUR128Summary(t *testing.T) {
	measurement, err := parseParityEBUR128Summary(`scanner progress I: -99.0 LUFS
Summary:

  Integrated loudness:
    I:         -21.1 LUFS
    Threshold: -31.6 LUFS

  True peak:
    Peak:       -3.1 dBFS
`)
	if err != nil || measurement.IntegratedLUFS != -21.1 || measurement.TruePeakDBFS != -3.1 {
		t.Fatalf("measurement=%+v err=%v", measurement, err)
	}
	thresholds := DefaultParityThresholds()
	metric := ParityAudioMetric{Pass: true}
	ApplyParityEBUR128(&metric, measurement, ParityEBUR128Measurement{IntegratedLUFS: -21.2, TruePeakDBFS: -3.2}, thresholds)
	if !metric.EBUR128Measured || !metric.Pass || math.Abs(metric.IntegratedLUFSDifference-.1) > 1e-9 || math.Abs(metric.TruePeakDBFSDifference-.1) > 1e-9 {
		t.Fatalf("threshold-boundary EBU metric=%+v", metric)
	}
	ApplyParityEBUR128(&metric, measurement, ParityEBUR128Measurement{IntegratedLUFS: -21.3, TruePeakDBFS: -3.1}, thresholds)
	if metric.Pass {
		t.Fatalf("out-of-tolerance loudness should fail: %+v", metric)
	}
}

func TestRendererDiagnosticFrameEncodesSingleFrame(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is unavailable")
	}
	frameIndex := int64(45)
	doc := NewEmptyTimeline(160, 90, 30)
	doc.DurationMS = 3000
	doc.Canvas.Background = "#204060"
	result, err := NewFFmpegRenderer(ffmpeg).Render(context.Background(), RenderRequest{
		Project:  models.VideoProject{ID: "parity-diagnostic", Width: doc.Canvas.Width, Height: doc.Canvas.Height, FPS: doc.Canvas.FPS, DurationMS: doc.DurationMS},
		Timeline: doc,
		Settings: ExportSettings{Format: "mp4", Codec: "h264", Resolution: "project", FPS: 30, Quality: "high", DiagnosticFrameIndex: &frameIndex},
	}, nil)
	if err != nil {
		t.Fatalf("render diagnostic frame: %v", err)
	}
	if result.DurationMS != 34 || len(result.Data) == 0 || result.MimeType != "image/png" {
		t.Fatalf("unexpected diagnostic result: duration=%d bytes=%d", result.DurationMS, len(result.Data))
	}
	command, _ := result.Metadata["ffmpeg_command"].(string)
	if !strings.Contains(command, "-frames:v 1") || !strings.Contains(command, "-ss 1.500000000") {
		t.Fatalf("diagnostic command omitted bounded frame selection: %s", command)
	}
	imageValue, err := png.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("decode PNG in Go: %v", err)
	}
	if imageValue.Bounds().Dx() != 160 || imageValue.Bounds().Dy() != 90 {
		t.Fatalf("diagnostic PNG dimensions = %v", imageValue.Bounds())
	}
}

func TestFilterTimelineAtDiagnosticFrameUsesCanonicalBoundaries(t *testing.T) {
	doc := NewEmptyTimeline(160, 90, 30)
	doc.DurationMS = 2000
	doc.Tracks = []TimelineTrack{{ID: "track", Type: TrackTypeLayer, Name: "Track", Visible: true, Clips: []TimelineClip{
		{ID: "before", StartMS: 0, DurationMS: 1000, TrimOutMS: 1000, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
		{ID: "at-boundary", StartMS: 1000, DurationMS: 1000, TrimOutMS: 1000, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
	}}}
	filtered := FilterTimelineAtDiagnosticFrame(doc, 30, 30, 0)
	if len(filtered.Tracks[0].Clips) != 1 || filtered.Tracks[0].Clips[0].ID != "at-boundary" {
		t.Fatalf("boundary clips = %+v", filtered.Tracks[0].Clips)
	}
	if filtered.DurationMS != doc.DurationMS {
		t.Fatalf("diagnostic filter changed timeline clock: %d", filtered.DurationMS)
	}
}

func TestFilterTimelineAtDiagnosticFrameIncludesSubFrameAndRangeRelativeStarts(t *testing.T) {
	doc := NewEmptyTimeline(160, 90, 120)
	doc.DurationMS = 2000
	doc.Tracks = []TimelineTrack{{ID: "track", Type: TrackTypeLayer, Name: "Track", Visible: true, Clips: []TimelineClip{
		{ID: "range-subframe", StartMS: 1005, DurationMS: 5, TrimOutMS: 5, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
		{ID: "next-frame", StartMS: 1010, DurationMS: 5, TrimOutMS: 5, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}},
	}}}
	filtered := FilterTimelineAtDiagnosticFrame(doc, 0, 120, 1000)
	if len(filtered.Tracks[0].Clips) != 1 || filtered.Tracks[0].Clips[0].ID != "range-subframe" {
		t.Fatalf("range-relative frame zero clips = %+v", filtered.Tracks[0].Clips)
	}
}

func TestFilterTimelineAtDiagnosticFrameSelectsOneSyntheticSamplePerParent(t *testing.T) {
	doc := NewEmptyTimeline(160, 90, 120)
	doc.DurationMS = 80
	doc.Tracks = []TimelineTrack{{ID: "track", Type: TrackTypeLayer, Name: "Track", Visible: true, Clips: []TimelineClip{{
		ID: "sampled", StartMS: 5, DurationMS: 50, TrimOutMS: 50,
		Effects: []TimelineEffect{},
		Keyframes: []TimelineKeyframe{
			{ID: "x0", Property: "x", TimeMS: 0, Value: 0, Easing: "linear"},
			{ID: "x1", Property: "x", TimeMS: 50, Value: 50, Easing: "linear"},
		},
	}},
	}}
	expanded := ExpandTimelineForFidelity(doc, 120, 300)
	frameZero := FilterTimelineAtDiagnosticFrame(expanded, 0, 120, 0)
	if len(frameZero.Tracks[0].Clips) != 1 {
		t.Fatalf("frame zero synthetic samples = %+v", frameZero.Tracks[0].Clips)
	}
	kind, parentID := fidelityGeneratedIdentity(frameZero.Tracks[0].Clips[0])
	if kind != rendererFidelityKindSample || parentID != "sampled" {
		t.Fatalf("frame zero sample provenance kind=%q parent=%q", kind, parentID)
	}
	boundaryFrame := FilterTimelineAtDiagnosticFrame(expanded, 2, 120, 0)
	if len(boundaryFrame.Tracks[0].Clips) != 1 {
		t.Fatalf("overlapping synthetic samples double-stacked: %+v", boundaryFrame.Tracks[0].Clips)
	}
	if boundaryFrame.Tracks[0].Clips[0].StartMS != 5 {
		t.Fatalf("frame presentation selected sample start=%d, want 5", boundaryFrame.Tracks[0].Clips[0].StartMS)
	}
}

func TestDiagnosticAudioGraphExcludesVisualFilters(t *testing.T) {
	volume := 1.25
	doc := NewEmptyTimeline(160, 90, 30)
	doc.Metadata = map[string]any{}
	graph, label := buildAudioFilterComplex(doc, []resolvedClip{{inputIdx: 1, hasAudio: true, clip: TimelineClip{StartMS: 250, DurationMS: 1000, TrimOutMS: 1000, Volume: &volume, FadeInMS: 100, Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{}}}})
	if label != "[diagnostic_audio]" || !strings.Contains(graph, "atrim=") || !strings.Contains(graph, "adelay=250|250") || !strings.Contains(graph, "asetpts=N/SR/TB,apad=pad_dur=") {
		t.Fatalf("audio graph = %q label=%q", graph, label)
	}
	for _, visual := range []string{"overlay=", "drawtext=", "drawbox=", "scale="} {
		if strings.Contains(graph, visual) {
			t.Fatalf("audio graph contains visual filter %q: %s", visual, graph)
		}
	}
}

func TestRendererDiagnosticAudioPadsToExactTimelineSampleCount(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is unavailable")
	}
	attachmentsDir := t.TempDir()
	audioPath := filepath.Join(attachmentsDir, "short.wav")
	cmd := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=0.5",
		"-c:a", "pcm_s16le", audioPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create short audio fixture: %v: %s", err, output)
	}

	doc := NewEmptyTimeline(160, 90, 30)
	doc.DurationMS = 1500
	doc.Tracks = []TimelineTrack{{
		ID: "audio", Type: TrackTypeAudio, Name: "Audio", Visible: true,
		Clips: []TimelineClip{{
			ID: "short-audio", AssetID: "short", StartMS: 250, DurationMS: 1000,
			TrimInMS: 0, TrimOutMS: 1000, PlaybackRate: 1,
			Effects: []TimelineEffect{}, Keyframes: []TimelineKeyframe{},
		}},
	}}
	result, err := NewFFmpegRenderer(ffmpeg).Render(context.Background(), RenderRequest{
		Project:  models.VideoProject{ID: "diagnostic-audio-padding", Width: 160, Height: 90, FPS: 30, DurationMS: doc.DurationMS},
		Timeline: doc,
		Settings: ExportSettings{Format: "mp4", Resolution: "project", FPS: 30, Quality: "high", IncludeAudio: true, DiagnosticAudio: true},
		Assets: map[string]models.VideoAsset{
			"short": {ID: "short", FilePath: audioPath, MimeType: "audio/wav"},
		},
		AttachmentsDir: attachmentsDir,
	}, nil)
	if err != nil {
		t.Fatalf("render diagnostic audio: %v", err)
	}
	const expectedBytes = 1500 * 48 * 2 * 2
	if len(result.Data) != expectedBytes {
		t.Fatalf("diagnostic PCM bytes = %d, want %d", len(result.Data), expectedBytes)
	}
}

func TestProbeParityDeliveryCountsDecodedFramesAndPTS(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is unavailable")
	}
	mediaPath := filepath.Join(t.TempDir(), "delivery.mp4")
	cmd := exec.Command(ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=160x90:r=10:d=1.2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", mediaPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create delivery fixture: %v: %s", err, output)
	}
	metric, err := ProbeParityDelivery(context.Background(), ffprobe, mediaPath, 1200, 10)
	if err != nil {
		t.Fatalf("probe delivery: %v", err)
	}
	if !metric.Pass || metric.ActualFrameCount != 12 || metric.ActualStartTimeSeconds != 0 || metric.ActualDurationSeconds != 1.2 {
		t.Fatalf("unexpected delivery metric: %+v", metric)
	}
}

func TestRenderDiagnosticFrameUsesImmutableSnapshot(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is unavailable")
	}
	database, err := db.Open(filepath.Join(t.TempDir(), "parity-diagnostic.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	projects := repository.NewVideoProjectRepo(database)
	generations := repository.NewVideoGenerationRepo(database)
	assets := repository.NewVideoAssetRepo(database)
	timelines := repository.NewVideoTimelineRepo(database)
	renderJobs := repository.NewVideoRenderJobRepo(database)
	service := NewService(projects, generations, assets, timelines, renderJobs, nil, t.TempDir(), nil)
	service.renderer = NewFFmpegRenderer(ffmpeg)

	project, err := service.CreateProject("", "Parity diagnostic", "openrouter", "fixture", 160, 90, 30, "16:9")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	doc := NewEmptyTimeline(160, 90, 30)
	doc.DurationMS = 3000
	doc.Canvas.Background = "#31557a"
	timeline, validated, err := service.SaveTimeline(context.Background(), "", project.ID, doc)
	if err != nil {
		t.Fatalf("save timeline: %v", err)
	}
	timelineJSON, _ := json.Marshal(validated)
	settingsJSON, _ := json.Marshal(ExportSettings{Format: "mp4", Resolution: "project", FPS: 30, Quality: "standard", IncludeAudio: true})
	manifestJSON := "[]"
	snapshot := &models.VideoRenderSnapshot{
		ID: "parity-snapshot", ProjectID: project.ID, TimelineID: timeline.ID,
		TimelineRevision: timeline.Revision, TimelineJSON: string(timelineJSON), TimelineSHA256: contentSHA256(timelineJSON),
		AssetManifestJSON: manifestJSON, AssetManifestSHA256: contentSHA256([]byte(manifestJSON)),
		SettingsJSON: string(settingsJSON), RenderContractVersion: 1, Renderer: "ffmpeg", RendererVersion: "test",
	}
	job := &models.VideoRenderJob{ID: "parity-job", ProjectID: project.ID, TimelineID: timeline.ID, Status: "completed", SettingsJSON: string(settingsJSON)}
	if err := renderJobs.CreateWithSnapshot(job, snapshot, nil); err != nil {
		t.Fatalf("create immutable snapshot: %v", err)
	}

	frame, err := service.RenderDiagnosticFrame(context.Background(), "", snapshot.ID, 45)
	if err != nil {
		t.Fatalf("render snapshot frame: %v", err)
	}
	if frame.FrameIndex != 45 || frame.TimeMS != 1500 || frame.TimelineHash != snapshot.TimelineSHA256 || len(frame.PNG) == 0 {
		t.Fatalf("unexpected snapshot frame: %+v", frame)
	}
	if _, err := png.Decode(bytes.NewReader(frame.PNG)); err != nil {
		t.Fatalf("snapshot frame is not PNG: %v", err)
	}
	if _, err := service.RenderDiagnosticFrame(context.Background(), "", snapshot.ID, 90); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("out-of-range frame error = %v", err)
	}
	audio, err := service.RenderDiagnosticAudio(context.Background(), "", snapshot.ID)
	if err != nil {
		t.Fatalf("render snapshot audio: %v", err)
	}
	if audio.SampleRate != 48000 || audio.Channels != 2 || audio.SampleFormat != "s16le" || len(audio.PCM) != 3000*48*2*2 {
		t.Fatalf("unexpected diagnostic PCM: rate=%d channels=%d format=%s bytes=%d", audio.SampleRate, audio.Channels, audio.SampleFormat, len(audio.PCM))
	}
}
