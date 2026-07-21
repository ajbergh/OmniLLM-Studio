package video

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// TestRendererGoldenMedia generates its own deterministic source fixture so the
// repository does not need to carry binary media. It validates semantic golden
// properties after a real FFmpeg round trip: frame composition, dimensions,
// audio presence, non-silent samples, and duration. Pixel thresholds are used
// instead of encoded-file hashes because codec output differs across FFmpeg
// builds while the decoded visual contract remains stable.
func TestRendererGoldenMedia(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is unavailable")
	}

	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "golden-source.mp4")
	generate := exec.Command(
		ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=c=red:s=160x90:r=30:d=1",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=1",
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac",
		sourcePath,
	)
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate golden source: %v: %s", err, output)
	}

	projectID := "golden-project"
	duration := int64(1000)
	width, height := 160, 90
	fps := 30.0
	document := NewEmptyTimeline(320, 180, 30)
	document.DurationMS = duration
	document.Tracks = []TimelineTrack{
		{
			ID: "media", Type: TrackTypeLayer, Name: "Media", Visible: true,
			Clips: []TimelineClip{{
				ID: "source", AssetID: "source-asset", DurationMS: duration, TrimOutMS: duration,
				Transform: map[string]any{"x": 0.0, "y": 0.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0},
				Keyframes: []TimelineKeyframe{
					{ID: "scale-start", Property: "scale", TimeMS: 0, Value: 1, Easing: "ease-in-out"},
					{ID: "scale-end", Property: "scale", TimeMS: 1000, Value: 1.2, Easing: "ease-in-out"},
				},
				Cursor: &TimelineCursor{
					Visible: true, Scale: 1, Highlight: true, ClickRings: true,
					Events: []TimelineCursorEvent{
						{TimeMS: 0, X: 250, Y: 140},
						{TimeMS: 500, X: 260, Y: 145, Click: true},
					},
				},
			}},
		},
		{
			ID: "annotations", Type: TrackTypeLayer, Name: "Annotations", Visible: true,
			Clips: []TimelineClip{{
				ID: "box", StartMS: 0, DurationMS: duration, TrimOutMS: duration,
				Transform: map[string]any{"x": -110.0, "y": -55.0, "scale": 1.0, "rotation": 0.0, "opacity": 1.0},
				Shape:     &TimelineShape{Kind: ShapeKindRectangle, Width: 70, Height: 40, Stroke: "#00ff00", StrokeWidth: 4},
			}},
		},
	}

	asset := models.VideoAsset{
		ID: "source-asset", ProjectID: &projectID, SourceType: "upload", Kind: "video",
		FileName: filepath.Base(sourcePath), FilePath: sourcePath, MimeType: "video/mp4",
		DurationMS: &duration, Width: &width, Height: &height, FPS: &fps,
	}
	request := RenderRequest{
		Project:        models.VideoProject{ID: projectID, Width: 320, Height: 180, FPS: 30, DurationMS: duration},
		Timeline:       document,
		Settings:       ExportSettings{Format: "mp4", Resolution: "project", FPS: 30, Quality: "draft", IncludeAudio: true},
		AttachmentsDir: directory,
		Assets:         map[string]models.VideoAsset{"source-asset": asset},
	}
	result, err := NewFidelityRenderer(NewFFmpegRenderer(ffmpeg)).Render(context.Background(), request, nil)
	if err != nil {
		t.Fatalf("render golden media: %v", err)
	}
	if len(result.Data) == 0 || result.Width != 320 || result.Height != 180 || result.DurationMS != duration {
		t.Fatalf("unexpected render result: bytes=%d size=%dx%d duration=%d", len(result.Data), result.Width, result.Height, result.DurationMS)
	}
	outputPath := filepath.Join(directory, "rendered.mp4")
	if err := os.WriteFile(outputPath, result.Data, 0o600); err != nil {
		t.Fatal(err)
	}

	probeOutput, err := exec.Command(
		ffprobe, "-v", "error", "-show_entries", "stream=codec_type,width,height", "-show_entries", "format=duration", "-of", "json", outputPath,
	).Output()
	if err != nil {
		t.Fatalf("probe rendered media: %v", err)
	}
	var probe struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(probeOutput, &probe); err != nil {
		t.Fatalf("decode probe output: %v", err)
	}
	hasVideo, hasAudio := false, false
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			hasVideo = stream.Width == 320 && stream.Height == 180
		case "audio":
			hasAudio = true
		}
	}
	if !hasVideo || !hasAudio {
		t.Fatalf("expected 320x180 video and audio streams: %s", probeOutput)
	}

	frame, err := exec.Command(
		ffmpeg, "-hide_banner", "-loglevel", "error", "-ss", "0.50", "-i", outputPath,
		"-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "-",
	).Output()
	if err != nil {
		t.Fatalf("decode golden frame: %v", err)
	}
	expectedFrameBytes := 320 * 180 * 3
	if len(frame) != expectedFrameBytes {
		t.Fatalf("unexpected decoded frame size: %d", len(frame))
	}
	pixel := func(x, y int) (byte, byte, byte) {
		offset := (y*320 + x) * 3
		return frame[offset], frame[offset+1], frame[offset+2]
	}
	red, green, blue := pixel(160, 90)
	if red < 180 || green > 80 || blue > 80 {
		t.Fatalf("center pixel does not match red source composition: rgb(%d,%d,%d)", red, green, blue)
	}
	cornerR, cornerG, cornerB := pixel(5, 5)
	if int(cornerR)+int(cornerG)+int(cornerB) > 80 {
		t.Fatalf("corner pixel should remain dark background: rgb(%d,%d,%d)", cornerR, cornerG, cornerB)
	}

	audio, err := exec.Command(
		ffmpeg, "-hide_banner", "-loglevel", "error", "-i", outputPath,
		"-map", "0:a:0", "-t", "0.25", "-f", "s16le", "-ac", "1", "-ar", "8000", "-",
	).Output()
	if err != nil {
		t.Fatalf("decode golden audio: %v", err)
	}
	if len(audio) == 0 || bytes.Equal(audio, make([]byte, len(audio))) {
		t.Fatal("rendered audio is empty or silent")
	}

	if command, _ := result.Metadata["ffmpeg_command"].(string); command == "" {
		t.Fatal("renderer metadata omitted FFmpeg command diagnostics")
	}
}
