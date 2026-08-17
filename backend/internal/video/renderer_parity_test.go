package video

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
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

	report := BuildParityReport("test-fixture", "timeline-hash", "manifest-hash", []ParityFramePair{pair}, &audio, thresholds)
	output := t.TempDir()
	if err := WriteParityReport(output, report, []ParityFramePair{pair}); err != nil {
		t.Fatalf("write parity artifacts: %v", err)
	}
	for _, name := range []string{"parity-report.json", "parity-report.md", filepath.Join("frames", "exact-side-by-side.png"), filepath.Join("frames", "exact-heatmap.png")} {
		if info, err := os.Stat(filepath.Join(output, name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing parity artifact %s: %v", name, err)
		}
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
