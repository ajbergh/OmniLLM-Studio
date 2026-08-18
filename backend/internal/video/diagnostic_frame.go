package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/video/rendercontract"
)

// DiagnosticFrame is a lossless PNG sampled from an immutable render
// snapshot. The image bytes are returned separately by the HTTP handler.
type DiagnosticFrame struct {
	SnapshotID   string
	FrameIndex   int64
	TimeMS       int64
	TimelineFPS  int
	TimelineHash string
	ManifestHash string
	PNG          []byte
}

type DiagnosticAudio struct {
	SnapshotID   string
	SampleRate   int
	Channels     int
	SampleFormat string
	TimelineHash string
	ManifestHash string
	PCM          []byte
}

// RenderDiagnosticFrame renders only one zero-based output frame while
// preserving the original timeline clock. It never consults mutable timeline
// or asset rows; the same immutable snapshot used by the delivery job is the
// sole source of render input.
func (s *Service) RenderDiagnosticFrame(ctx context.Context, userID, snapshotID string, frameIndex int64) (*DiagnosticFrame, error) {
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return nil, fmt.Errorf("render snapshot id is required")
	}
	if frameIndex < 0 {
		return nil, fmt.Errorf("diagnostic frame index must be non-negative")
	}
	snapshot, err := s.renderJobs.GetSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("render snapshot not found")
	}
	project, err := s.ensureProjectOwned(userID, snapshot.ProjectID)
	if err != nil {
		return nil, err
	}
	if contentSHA256([]byte(snapshot.TimelineJSON)) != snapshot.TimelineSHA256 {
		return nil, fmt.Errorf("render snapshot timeline hash mismatch")
	}
	doc, err := TimelineFromJSON(snapshot.TimelineJSON, NewEmptyTimeline(project.Width, project.Height, project.FPS))
	if err != nil {
		return nil, err
	}
	var settings ExportSettings
	if err := json.Unmarshal([]byte(snapshot.SettingsJSON), &settings); err != nil {
		return nil, fmt.Errorf("invalid render snapshot settings: %w", err)
	}
	settings, err = validateExportSettings(settings, *project)
	if err != nil {
		return nil, fmt.Errorf("invalid render snapshot settings: %w", err)
	}
	fps := settings.FPS
	durationMS := doc.DurationMS
	if settings.RangeEndMS > settings.RangeStartMS {
		durationMS = minInt64(doc.DurationMS, settings.RangeEndMS) - settings.RangeStartMS
	}
	totalFrames := rendercontract.FrameCount(durationMS, fps)
	if frameIndex >= totalFrames {
		return nil, fmt.Errorf("diagnostic frame index %d is outside [0,%d)", frameIndex, totalFrames)
	}
	settings.Format = "mp4"
	settings.Codec = "h264"
	settings.Quality = "high"
	settings.IncludeAudio = false
	settings.SidecarCaptions = ""
	settings.DiagnosticFrameIndex = &frameIndex

	assets, err := s.assetsFromRenderSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	result, err := s.renderer.Render(ctx, RenderRequest{
		Project: *project, Timeline: doc, Settings: settings,
		AttachmentsDir: s.attachmentsDir, Assets: assets,
	}, nil)
	if err != nil {
		return nil, err
	}
	pngBytes := result.Data
	if result.MimeType != "image/png" {
		pngBytes, err = decodeFirstFramePNG(ctx, result.Data)
		if err != nil {
			return nil, err
		}
	}
	return &DiagnosticFrame{
		SnapshotID: snapshot.ID, FrameIndex: frameIndex,
		TimeMS:      int64(math.Round(float64(frameIndex) * 1000 / float64(fps))),
		TimelineFPS: fps, TimelineHash: snapshot.TimelineSHA256,
		ManifestHash: snapshot.AssetManifestSHA256, PNG: pngBytes,
	}, nil
}

// RenderDiagnosticAudio emits headerless signed-16 stereo PCM for the exact
// immutable snapshot mix and submitted range. Snapshots submitted without
// audio remain silent-by-contract and therefore reject this diagnostic.
func (s *Service) RenderDiagnosticAudio(ctx context.Context, userID, snapshotID string) (*DiagnosticAudio, error) {
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return nil, fmt.Errorf("render snapshot id is required")
	}
	snapshot, err := s.renderJobs.GetSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, fmt.Errorf("render snapshot not found")
	}
	project, err := s.ensureProjectOwned(userID, snapshot.ProjectID)
	if err != nil {
		return nil, err
	}
	if contentSHA256([]byte(snapshot.TimelineJSON)) != snapshot.TimelineSHA256 {
		return nil, fmt.Errorf("render snapshot timeline hash mismatch")
	}
	doc, err := TimelineFromJSON(snapshot.TimelineJSON, NewEmptyTimeline(project.Width, project.Height, project.FPS))
	if err != nil {
		return nil, err
	}
	var settings ExportSettings
	if err := json.Unmarshal([]byte(snapshot.SettingsJSON), &settings); err != nil {
		return nil, fmt.Errorf("invalid render snapshot settings: %w", err)
	}
	settings, err = validateExportSettings(settings, *project)
	if err != nil {
		return nil, fmt.Errorf("invalid render snapshot settings: %w", err)
	}
	if !settings.IncludeAudio {
		return nil, fmt.Errorf("render snapshot excludes audio")
	}
	settings.Format, settings.Codec, settings.SidecarCaptions = "mp4", "h264", ""
	settings.DiagnosticAudio = true
	assets, err := s.assetsFromRenderSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	result, err := s.renderer.Render(ctx, RenderRequest{Project: *project, Timeline: doc, Settings: settings, AttachmentsDir: s.attachmentsDir, Assets: assets}, nil)
	if err != nil {
		return nil, err
	}
	if result.MimeType != "audio/pcm" || len(result.Data) == 0 {
		return nil, fmt.Errorf("renderer returned an invalid diagnostic audio result")
	}
	return &DiagnosticAudio{SnapshotID: snapshot.ID, SampleRate: 48000, Channels: 2, SampleFormat: "s16le", TimelineHash: snapshot.TimelineSHA256, ManifestHash: snapshot.AssetManifestSHA256, PCM: result.Data}, nil
}

func decodeFirstFramePNG(ctx context.Context, encoded []byte) ([]byte, error) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg was not found in PATH; install FFmpeg to decode diagnostic frames")
	}
	cmd := exec.CommandContext(ctx, binary,
		"-hide_banner", "-loglevel", "error", "-i", "pipe:0",
		"-frames:v", "1", "-f", "image2pipe", "-vcodec", "png", "pipe:1",
	)
	cmd.Stdin = bytes.NewReader(encoded)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("decode diagnostic frame: %w: %s", err, responseSnippet(stderr.Bytes()))
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("decode diagnostic frame: ffmpeg returned an empty PNG")
	}
	return output, nil
}
