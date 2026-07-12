// FFmpeg-backed timeline renderer. Builds a single filter_complex graph that
// composites media, text, and annotation clips in layer order (matching the
// preview), mixes audio with per-clip volume/fade/keyframe envelopes, and
// encodes via libx264/libx265/libvpx-vp9. Export settings can slice a
// timeline range and strip caption burn-in before the graph is built.
//
// Every supported/partial/skipped feature here must be reflected in
// renderer_capabilities.go — that matrix drives the editor's export-fidelity
// warnings, so an unreported gap is a silent lie to the user. FFmpeg
// arguments are constructed as discrete argv entries; never interpolate
// untrusted strings into a shell command.

package video

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

type Renderer interface {
	Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error)
}

type RenderRequest struct {
	Project  models.VideoProject `json:"project"`
	Timeline TimelineDocument    `json:"timeline"`
	Settings ExportSettings      `json:"settings"`
	// AttachmentsDir is the root directory where asset files are stored.
	// Required for media compositing; if empty, only text overlays are rendered.
	AttachmentsDir string `json:"-"`
	// Assets maps asset ID → VideoAsset for clips referenced in the timeline.
	Assets map[string]models.VideoAsset `json:"-"`
}

// resolvedClip is a timeline clip with its asset file path resolved to an absolute path.
type resolvedClip struct {
	inputIdx   int
	trackIndex int
	clip       TimelineClip
	filePath   string
	isVideo    bool
	isImage    bool
	isAudio    bool
	// hasAudio reports whether a video asset carries an audio stream that
	// should join the mixdown (always true for audio assets).
	hasAudio bool
}

// RenderError carries FFmpeg diagnostics for failed renders so they can be
// persisted in render job metadata.
type RenderError struct {
	Command string
	Stderr  string
	Err     error
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("ffmpeg render failed: %v: %s", e.Err, responseSnippet([]byte(e.Stderr)))
}

func (e *RenderError) Unwrap() error { return e.Err }

type RenderProgress struct {
	Stage    string  `json:"stage"`
	Message  string  `json:"message"`
	Progress float64 `json:"progress"`
}

type RenderResult struct {
	MimeType   string         `json:"mime_type"`
	FileName   string         `json:"file_name"`
	Data       []byte         `json:"-"`
	DurationMS int64          `json:"duration_ms"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	FPS        float64        `json:"fps"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type FFmpegRenderer struct {
	binary string
}

func NewFFmpegRenderer(binary string) *FFmpegRenderer {
	return &FFmpegRenderer{binary: strings.TrimSpace(binary)}
}

func (r *FFmpegRenderer) Render(ctx context.Context, req RenderRequest, progress func(RenderProgress)) (*RenderResult, error) {
	if req.Project.ID == "" {
		return nil, fmt.Errorf("project is required")
	}
	var err error
	req.Timeline, err = ValidateTimelineDocument(req.Timeline)
	if err != nil {
		return nil, err
	}
	// Export range: slice the validated document so the filtergraph only sees
	// the requested window (clip trims, keyframes, and markers rebase).
	if req.Settings.RangeEndMS > req.Settings.RangeStartMS && req.Settings.RangeStartMS >= 0 {
		req.Timeline = SliceTimelineRange(req.Timeline, req.Settings.RangeStartMS, req.Settings.RangeEndMS)
	}
	// Burn-in toggle: dropping caption-track clips keeps them out of the frame;
	// sidecar files are written by the render job from the original document.
	if req.Settings.BurnInCaptions != nil && !*req.Settings.BurnInCaptions {
		req.Timeline = StripCaptionOverlays(req.Timeline)
	}
	binary := r.binary
	if binary == "" {
		binary, err = exec.LookPath("ffmpeg")
		if err != nil {
			return nil, fmt.Errorf("ffmpeg was not found in PATH; install FFmpeg to render video exports")
		}
	}
	format := strings.ToLower(strings.TrimSpace(req.Settings.Format))
	if format == "" {
		format = "mp4"
	}
	if format != "mp4" && format != "webm" {
		return nil, fmt.Errorf("unsupported export format %q", format)
	}
	width, height := renderDimensions(req)
	fps := req.Settings.FPS
	if fps <= 0 {
		fps = req.Timeline.Canvas.FPS
	}
	if fps <= 0 {
		fps = DefaultProjectFPS
	}
	durationSeconds := float64(maxInt64(req.Timeline.DurationMS, 1000)) / 1000.0
	background := ffmpegColor(req.Timeline.Canvas.Background, "0x000000")

	if progress != nil {
		progress(RenderProgress{Stage: "preparing", Message: "Preparing FFmpeg timeline composition", Progress: 0.15})
	}
	tmp, err := os.CreateTemp("", "omnillm-video-render-*."+format)
	if err != nil {
		return nil, err
	}
	outputPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(outputPath)

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=%s:s=%dx%d:r=%d:d=%.3f", background, width, height, fps, durationSeconds),
	}

	// Attempt to resolve media clips from the asset map.
	resolved := resolveMediaClips(req)
	if !req.Settings.IncludeAudio {
		visualOnly := make([]resolvedClip, 0, len(resolved))
		for _, rc := range resolved {
			rc.hasAudio = false
			rc.isAudio = false
			if rc.isVideo || rc.isImage {
				visualOnly = append(visualOnly, rc)
			}
		}
		resolved = visualOnly
	}

	if len(resolved) > 0 {
		// ── Media compositing path ─────────────────────────────────────────
		// Assign input indices and add each file as an FFmpeg input.
		nextIdx := 1
		for i := range resolved {
			resolved[i].inputIdx = nextIdx
			args = append(args, "-i", resolved[i].filePath)
			nextIdx++
		}
		hasAudioClips := func() bool {
			for _, rc := range resolved {
				if rc.hasAudio {
					return true
				}
			}
			return false
		}()
		// Add silent audio source only when IncludeAudio is requested but no
		// audio clips are present in the timeline.
		anullIdx := -1
		if req.Settings.IncludeAudio && !hasAudioClips {
			anullIdx = nextIdx
			args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
		}

		filterStr, videoLabel, audioLabel := buildFilterComplexWithAudio(req.Timeline, resolved, width, height, req.Settings.IncludeAudio)
		args = append(args, "-filter_complex", filterStr)
		args = append(args, "-t", fmt.Sprintf("%.3f", durationSeconds), "-r", fmt.Sprintf("%d", fps))
		args = append(args, "-map", videoLabel)
		if req.Settings.IncludeAudio {
			if audioLabel != "" {
				args = append(args, "-map", audioLabel)
			} else if anullIdx >= 0 {
				args = append(args, "-map", fmt.Sprintf("%d:a:0", anullIdx), "-shortest")
			}
		}
	} else {
		// ── Simple path (no media files) ───────────────────────────────────
		if req.Settings.IncludeAudio {
			args = append(args, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000")
		}
		if filters := ffmpegVideoFilters(req.Timeline, width, height); filters != "" {
			args = append(args, "-vf", filters)
		}
		args = append(args, "-t", fmt.Sprintf("%.3f", durationSeconds), "-r", fmt.Sprintf("%d", fps), "-map", "0:v:0")
		if req.Settings.IncludeAudio {
			args = append(args, "-map", "1:a:0", "-shortest")
		}
	}
	args = appendFFmpegCodecArgs(args, format, req.Settings)
	args = append(args, outputPath)

	if progress != nil {
		progress(RenderProgress{Stage: "encoding", Message: "Encoding video export with FFmpeg", Progress: 0.65})
	}
	commandStr := "ffmpeg " + strings.Join(args, " ")
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &RenderError{Command: commandStr, Stderr: string(output), Err: err}
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read ffmpeg output: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("ffmpeg produced an empty export")
	}
	if progress != nil {
		progress(RenderProgress{Stage: "finalizing", Message: "Finalizing rendered video asset", Progress: 0.95})
	}
	mimeType := "video/mp4"
	if format == "webm" {
		mimeType = "video/webm"
	}
	return &RenderResult{
		MimeType:   mimeType,
		FileName:   fmt.Sprintf("render-%s.%s", sanitizePathSegment(req.Project.ID), format),
		Data:       data,
		DurationMS: int64(durationSeconds * 1000),
		Width:      width,
		Height:     height,
		FPS:        float64(fps),
		Metadata: map[string]any{
			"renderer":             "ffmpeg",
			"format":               format,
			"quality":              req.Settings.Quality,
			"include_audio":        req.Settings.IncludeAudio,
			"text_clips":           countTextClips(req.Timeline),
			"ffmpeg_command":       commandStr,
			"timeline_duration_ms": req.Timeline.DurationMS,
		},
	}, nil
}

func renderDimensions(req RenderRequest) (int, int) {
	if req.Settings.Width > 0 && req.Settings.Height > 0 {
		return evenDimension(req.Settings.Width), evenDimension(req.Settings.Height)
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Settings.Resolution))
	if resolution == "" || resolution == "project" || resolution == "custom" {
		width := req.Timeline.Canvas.Width
		height := req.Timeline.Canvas.Height
		if width <= 0 {
			width = req.Project.Width
		}
		if height <= 0 {
			height = req.Project.Height
		}
		if width <= 0 || height <= 0 {
			return DefaultProjectWidth, DefaultProjectHeight
		}
		return evenDimension(width), evenDimension(height)
	}
	width, height := dimensionsForResolution(resolution, aspectRatioForCanvas(req.Timeline.Canvas.Width, req.Timeline.Canvas.Height))
	return evenDimension(width), evenDimension(height)
}

func aspectRatioForCanvas(width, height int) string {
	if width <= 0 || height <= 0 {
		return DefaultAspectRatio
	}
	switch {
	case width == height:
		return "1:1"
	case height > width:
		return "9:16"
	default:
		return "16:9"
	}
}

func evenDimension(value int) int {
	if value <= 0 {
		return 2
	}
	if value%2 == 1 {
		return value + 1
	}
	return value
}

func appendFFmpegCodecArgs(args []string, format string, settings ExportSettings) []string {
	audioBitrate := "128k"
	if settings.AudioBitrateKbps >= 32 && settings.AudioBitrateKbps <= 512 {
		audioBitrate = fmt.Sprintf("%dk", settings.AudioBitrateKbps)
	}
	switch format {
	case "webm":
		crf := "34"
		if settings.Quality == "high" {
			crf = "28"
		} else if settings.Quality == "draft" {
			crf = "40"
		}
		args = append(args, "-c:v", "libvpx-vp9", "-b:v", "0", "-crf", crf, "-pix_fmt", "yuv420p")
		if settings.IncludeAudio {
			args = append(args, "-c:a", "libopus", "-b:a", audioBitrate)
		}
	default:
		if settings.Codec == "h265" {
			// H.265 needs an ffmpeg built with libx265; failures surface in the
			// job's FFmpeg diagnostics rather than being silently downgraded.
			crf := "28"
			if settings.Quality == "high" {
				crf = "22"
			} else if settings.Quality == "draft" {
				crf = "34"
			}
			args = append(args, "-c:v", "libx265", "-preset", "fast", "-crf", crf, "-pix_fmt", "yuv420p", "-tag:v", "hvc1", "-movflags", "+faststart")
		} else {
			crf := "23"
			if settings.Quality == "high" {
				crf = "18"
			} else if settings.Quality == "draft" {
				crf = "30"
			}
			args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", crf, "-pix_fmt", "yuv420p", "-movflags", "+faststart")
		}
		if settings.IncludeAudio {
			args = append(args, "-c:a", "aac", "-b:a", audioBitrate)
		}
	}
	return args
}

// resolveMediaClips iterates the timeline tracks and returns all clips that
// reference a known asset with a valid file on disk, in track stacking order
// (later tracks composite on top, matching the preview), then z-index, then
// start time.
func resolveMediaClips(req RenderRequest) []resolvedClip {
	if req.AttachmentsDir == "" || len(req.Assets) == 0 {
		return nil
	}
	var result []resolvedClip
	// One audio-stream lookup per asset, not per clip.
	audioProbeCache := map[string]bool{}
	for trackIndex, track := range req.Timeline.Tracks {
		for _, clip := range track.Clips {
			if clip.AssetID == "" || clip.DurationMS <= 0 {
				continue
			}
			asset, ok := req.Assets[clip.AssetID]
			if !ok || asset.FilePath == "" {
				continue
			}
			fullPath := filepath.Join(req.AttachmentsDir, filepath.FromSlash(asset.FilePath))
			if _, err := os.Stat(fullPath); err != nil {
				continue // file not found on disk — skip silently
			}
			mime := strings.ToLower(strings.TrimSpace(strings.SplitN(asset.MimeType, ";", 2)[0]))
			rc := resolvedClip{
				trackIndex: trackIndex,
				clip:       clip,
				filePath:   fullPath,
				isVideo:    strings.HasPrefix(mime, "video/"),
				isImage:    strings.HasPrefix(mime, "image/"),
				isAudio:    strings.HasPrefix(mime, "audio/"),
			}
			if rc.isAudio {
				rc.hasAudio = true
			} else if rc.isVideo {
				if cached, ok := audioProbeCache[clip.AssetID]; ok {
					rc.hasAudio = cached
				} else {
					rc.hasAudio = videoAssetHasAudio(asset, fullPath)
					audioProbeCache[clip.AssetID] = rc.hasAudio
				}
			}
			// An audio-only clip turns a video asset into a detached audio clip.
			if rc.clip.AudioOnly {
				rc.isVideo, rc.isImage = false, false
			}
			// Hidden tracks contribute no video; muted tracks (and muted clips)
			// contribute no audio.
			if (rc.isVideo || rc.isImage) && !track.Visible {
				// Video clips on hidden tracks still contribute their audio.
				rc.isVideo, rc.isImage = false, false
			}
			if track.Muted || rc.clip.Muted {
				rc.hasAudio = false
			}
			if rc.isVideo || rc.isImage || rc.hasAudio {
				result = append(result, rc)
			}
		}
	}
	// Layer order controls visual stacking; start time only controls when a
	// clip is enabled. Sorting by start time here would let an early clip on a
	// top layer composite below a later clip on a bottom layer.
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].trackIndex != result[j].trackIndex {
			return result[i].trackIndex < result[j].trackIndex
		}
		zi, zj := clipZIndex(result[i].clip), clipZIndex(result[j].clip)
		if zi != zj {
			return zi < zj
		}
		return result[i].clip.StartMS < result[j].clip.StartMS
	})
	return result
}

func clipZIndex(clip TimelineClip) int {
	if clip.ZIndex != nil {
		return *clip.ZIndex
	}
	return 0
}

// videoAssetHasAudio reports whether a video asset carries an audio stream.
// Prefers the has_audio flag probed at ingest; falls back to a one-off
// ffprobe. Defaults to false when neither source is conclusive so the filter
// graph never maps a missing audio stream (which would fail the render).
func videoAssetHasAudio(asset models.VideoAsset, fullPath string) bool {
	if strings.TrimSpace(asset.MetadataJSON) != "" {
		var meta struct {
			HasAudio *bool `json:"has_audio"`
		}
		if err := json.Unmarshal([]byte(asset.MetadataJSON), &meta); err == nil && meta.HasAudio != nil {
			return *meta.HasAudio
		}
	}
	probe, err := ProbeMedia(context.Background(), fullPath)
	if err != nil || probe == nil {
		return false
	}
	return probe.HasAudio
}

// buildFilterComplex constructs an audio-enabled FFmpeg -filter_complex
// expression that composites all resolved media clips onto a lavfi color
// background (input 0). It returns the graph plus final video/audio labels.
// Audio-disabled exports must instead call buildFilterComplexWithAudio with
// includeAudio=false so no unmapped labeled audio output is produced.
func buildFilterComplex(doc TimelineDocument, clips []resolvedClip, width, height int) (filterStr, videoLabel, audioLabel string) {
	return buildFilterComplexWithAudio(doc, clips, width, height, true)
}

// buildFilterComplexWithAudio lets audio-disabled exports omit audio filter
// outputs entirely. FFmpeg rejects a labeled filter output that is not mapped,
// so building the audio branch and then mapping video alone is not valid.
func buildFilterComplexWithAudio(doc TimelineDocument, clips []resolvedClip, width, height int, includeAudio bool) (filterStr, videoLabel, audioLabel string) {
	var parts []string

	// ── Visual chain: media overlays and text interleaved in layer order ────
	// One ordered list so a text clip on a lower layer composites beneath
	// media on a higher layer, matching the preview compositor. Mute only
	// silences audio; hidden tracks drop their text and visuals.
	type visualItem struct {
		media      *resolvedClip // nil for text items
		clip       TimelineClip
		trackIndex int
	}
	var items []visualItem
	for i := range clips {
		if clips[i].isVideo || clips[i].isImage {
			items = append(items, visualItem{media: &clips[i], clip: clips[i].clip, trackIndex: clips[i].trackIndex})
		}
	}
	for trackIndex, track := range doc.Tracks {
		if !track.Visible {
			continue
		}
		for _, clip := range track.Clips {
			hasText := clip.Text != nil && strings.TrimSpace(clip.Text.Text) != ""
			if !hasText && clip.Shape == nil {
				continue
			}
			items = append(items, visualItem{clip: clip, trackIndex: trackIndex})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].trackIndex != items[j].trackIndex {
			return items[i].trackIndex < items[j].trackIndex
		}
		zi, zj := clipZIndex(items[i].clip), clipZIndex(items[j].clip)
		if zi != zj {
			return zi < zj
		}
		return items[i].clip.StartMS < items[j].clip.StartMS
	})

	prevV := "[base_v]"
	parts = append(parts, "[0:v]setpts=PTS-STARTPTS[base_v]")
	textIdx := 0

	for _, item := range items {
		if item.media == nil {
			// A callout can carry both a shape and a label: the box composites
			// beneath its own text.
			if item.clip.Shape != nil && (item.clip.Shape.Kind == ShapeKindBlur || item.clip.Shape.Kind == ShapeKindPixelate) {
				if blurParts, outLabel := blurRegionParts(prevV, item.clip, *item.clip.Shape, width, height, textIdx); len(blurParts) > 0 {
					parts = append(parts, blurParts...)
					prevV = outLabel
					textIdx++
				}
			} else if item.clip.Shape != nil {
				if filter := drawBoxFilter(item.clip, *item.clip.Shape, width, height); filter != "" {
					outLabel := fmt.Sprintf("[t%d_v]", textIdx)
					parts = append(parts, prevV+filter+outLabel)
					prevV = outLabel
					textIdx++
				}
			}
			if item.clip.Text != nil && strings.TrimSpace(item.clip.Text.Text) != "" {
				if filter := drawTextFilter(item.clip, *item.clip.Text, width, height); filter != "" {
					outLabel := fmt.Sprintf("[t%d_v]", textIdx)
					parts = append(parts, prevV+filter+outLabel)
					prevV = outLabel
					textIdx++
				}
			}
			continue
		}
		rc := *item.media
		cLabel := fmt.Sprintf("[c%d_v]", rc.inputIdx)
		ovLabel := fmt.Sprintf("[ov%d_v]", rc.inputIdx)
		startS := float64(rc.clip.StartMS) / 1000.0
		endS := float64(rc.clip.StartMS+rc.clip.DurationMS) / 1000.0
		trimInS := float64(rc.clip.TrimInMS) / 1000.0
		durS := float64(rc.clip.DurationMS) / 1000.0
		sourceDurS := float64(sourceDurationFor(rc.clip, rc.clip.DurationMS)) / 1000.0
		playbackRate := clipPlaybackRate(rc.clip)
		tr := parseClipTransform(rc.clip.Transform)

		var chain []string
		if rc.isImage {
			chain = append(chain, "loop=loop=-1:size=1:start=0", fmt.Sprintf("trim=duration=%.3f", durS), "setpts=PTS-STARTPTS")
		} else {
			chain = append(chain, fmt.Sprintf("trim=start=%.3f:duration=%.3f", trimInS, sourceDurS))
			if playbackRate == 1 {
				chain = append(chain, "setpts=PTS-STARTPTS")
			} else {
				chain = append(chain, fmt.Sprintf("setpts=(PTS-STARTPTS)/%.6f", playbackRate))
			}
		}
		if tr.hasCrop {
			chain = append(chain, fmt.Sprintf("crop=iw*%.4f:ih*%.4f:iw*%.4f:ih*%.4f",
				1-tr.cropLeft-tr.cropRight, 1-tr.cropTop-tr.cropBottom, tr.cropLeft, tr.cropTop))
		}
		scaledW := maxInt(2, int(float64(width)*tr.scale+0.5))
		scaledH := maxInt(2, int(float64(height)*tr.scale+0.5))
		chain = append(chain, fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", scaledW, scaledH))
		chain = append(chain, effectFilters(rc.clip.Effects)...)
		chain = append(chain, "format=rgba")
		if expr := positionKeyframeExpr(rc.clip.Keyframes, "rotation", 0); expr != "" {
			// rotate evaluates the angle per frame; after setpts the stream's
			// t is clip-relative, matching keyframe times. A fixed diagonal
			// bounding box keeps the output size constant while rotating.
			chain = append(chain, fmt.Sprintf("rotate=a='(%s)*PI/180':c=black@0:ow='hypot(iw\\,ih)':oh=ow", expr))
		} else if tr.rotation != 0 {
			// Rotate after format=rgba so the expanded corners stay transparent.
			rad := tr.rotation * math.Pi / 180.0
			chain = append(chain, fmt.Sprintf("rotate=%.6f:c=black@0:ow=rotw(%.6f):oh=roth(%.6f)", rad, rad, rad))
		}
		if tr.opacity < 1 {
			chain = append(chain, fmt.Sprintf("colorchannelmixer=aa=%.3f", tr.opacity))
		}
		fadeInS, fadeOutS := clipFadeSeconds(rc.clip)
		if fadeInS > 0 {
			chain = append(chain, fmt.Sprintf("fade=t=in:st=0:d=%.3f:alpha=1", fadeInS))
		}
		if fadeOutS > 0 {
			chain = append(chain, fmt.Sprintf("fade=t=out:st=%.3f:d=%.3f:alpha=1", durS-fadeOutS, fadeOutS))
		}

		srcFilter := fmt.Sprintf("[%d:v]%s%s", rc.inputIdx, strings.Join(chain, ","), cLabel)
		xExpr := fmt.Sprintf("(W-w)/2%+.0f", tr.x)
		if expr := positionKeyframeExpr(rc.clip.Keyframes, "x", startS); expr != "" {
			xExpr = "(W-w)/2+" + expr
		}
		yExpr := fmt.Sprintf("(H-h)/2%+.0f", tr.y)
		if expr := positionKeyframeExpr(rc.clip.Keyframes, "y", startS); expr != "" {
			yExpr = "(H-h)/2+" + expr
		}
		if slideX, slideY := slideTransitionExpr(rc.clip, startS, endS); slideX != "" || slideY != "" {
			if slideX != "" {
				xExpr = fmt.Sprintf("(%s)+(%s)", xExpr, slideX)
			}
			if slideY != "" {
				yExpr = fmt.Sprintf("(%s)+(%s)", yExpr, slideY)
			}
		}
		overlayFilter := fmt.Sprintf("%s%soverlay=enable='between(t\\,%.3f\\,%.3f)':x='%s':y='%s'%s",
			prevV, cLabel, startS, endS, xExpr, yExpr, ovLabel)

		parts = append(parts, srcFilter, overlayFilter)
		prevV = ovLabel
	}

	videoLabel = prevV

	// ── Audio clips ─────────────────────────────────────────────────────────
	// hasAudio covers audio assets and video assets with an audio stream;
	// muted tracks/clips were already dropped in resolveMediaClips.
	if includeAudio {
		var aLabels []string
		for _, rc := range clips {
			if !rc.hasAudio {
				continue
			}
			aLabel := fmt.Sprintf("[a%d]", rc.inputIdx)
			trimInS := float64(rc.clip.TrimInMS) / 1000.0
			durS := float64(rc.clip.DurationMS) / 1000.0
			sourceDurS := float64(sourceDurationFor(rc.clip, rc.clip.DurationMS)) / 1000.0
			startMS := rc.clip.StartMS
			chain := []string{
				fmt.Sprintf("atrim=start=%.3f:duration=%.3f", trimInS, sourceDurS),
				"asetpts=PTS-STARTPTS",
			}
			chain = append(chain, atempoFilters(clipPlaybackRate(rc.clip))...)
			// Volume keyframes override the static volume (matching the preview).
			// The retimed stream starts at 0, so keyframe time is `t` directly.
			if expr := positionKeyframeExpr(rc.clip.Keyframes, "volume", 0); expr != "" {
				chain = append(chain, fmt.Sprintf("volume=volume='%s':eval=frame", expr))
			} else if rc.clip.Volume != nil && *rc.clip.Volume >= 0 && *rc.clip.Volume != 1 {
				chain = append(chain, fmt.Sprintf("volume=%.3f", clampFloat(*rc.clip.Volume, 0, 2)))
			}
			fadeInS, fadeOutS := clipFadeSeconds(rc.clip)
			if fadeInS > 0 {
				chain = append(chain, fmt.Sprintf("afade=t=in:st=0:d=%.3f", fadeInS))
			}
			if fadeOutS > 0 {
				chain = append(chain, fmt.Sprintf("afade=t=out:st=%.3f:d=%.3f", durS-fadeOutS, fadeOutS))
			}
			chain = append(chain, fmt.Sprintf("adelay=%d|%d", startMS, startMS))
			parts = append(parts, fmt.Sprintf("[%d:a]%s%s", rc.inputIdx, strings.Join(chain, ","), aLabel))
			aLabels = append(aLabels, aLabel)
		}
		if len(aLabels) > 1 {
			audioLabel = "[final_audio]"
			parts = append(parts, strings.Join(aLabels, "")+
				fmt.Sprintf("amix=inputs=%d:normalize=0:dropout_transition=0[final_audio]", len(aLabels)))
		} else if len(aLabels) == 1 {
			audioLabel = aLabels[0]
		}
	}

	return strings.Join(parts, ";"), videoLabel, audioLabel
}

func ffmpegVideoFilters(doc TimelineDocument, width, height int) string {
	var filters []string
	for _, track := range doc.Tracks {
		if !track.Visible {
			continue
		}
		for _, clip := range track.Clips {
			if clip.Shape != nil {
				if filter := drawBoxFilter(clip, *clip.Shape, width, height); filter != "" {
					filters = append(filters, filter)
				}
			}
			if clip.Text == nil || strings.TrimSpace(clip.Text.Text) == "" {
				continue
			}
			filter := drawTextFilter(clip, *clip.Text, width, height)
			if filter != "" {
				filters = append(filters, filter)
			}
		}
	}
	return strings.Join(filters, ",")
}

// blurRegionParts blurs the composited frame beneath a blur-region shape:
// the chain so far splits, the region crops out and blurs, and the blurred
// patch overlays back at the same position during the clip's window. Returns
// the graph parts and the chain's new output label.
func blurRegionParts(prev string, clip TimelineClip, shape TimelineShape, width, height, idx int) ([]string, string) {
	tr := parseClipTransform(clip.Transform)
	boxW := shape.Width
	boxH := shape.Height
	if boxW <= 0 || boxH <= 0 {
		return nil, ""
	}
	if tr.scale != 1 {
		boxW = maxInt(2, int(float64(boxW)*tr.scale+0.5))
		boxH = maxInt(2, int(float64(boxH)*tr.scale+0.5))
	}
	// crop requires the region fully inside the frame.
	if boxW > width {
		boxW = width
	}
	if boxH > height {
		boxH = height
	}
	x := (width-boxW)/2 + int(tr.x)
	y := (height-boxH)/2 + int(tr.y)
	x = maxInt(0, minInt(x, width-boxW))
	y = maxInt(0, minInt(y, height-boxH))
	radius := int(shape.BlurRadius + 0.5)
	if radius <= 0 {
		radius = 12
	}
	startS := float64(clip.StartMS) / 1000.0
	endS := float64(clip.StartMS+clip.DurationMS) / 1000.0
	srcLabel := fmt.Sprintf("[bbs%d]", idx)
	baseLabel := fmt.Sprintf("[bbb%d]", idx)
	blurLabel := fmt.Sprintf("[bbl%d]", idx)
	outLabel := fmt.Sprintf("[t%d_v]", idx)
	if shape.Kind == ShapeKindPixelate {
		// Mosaic redaction: shrink the region by the block size and scale it
		// back up with nearest-neighbor so each block becomes one flat cell.
		block := maxInt(2, radius)
		downW := maxInt(1, boxW/block)
		downH := maxInt(1, boxH/block)
		return []string{
			prev + "split" + srcLabel + baseLabel,
			fmt.Sprintf("%scrop=%d:%d:%d:%d,scale=%d:%d,scale=%d:%d:flags=neighbor%s", srcLabel, boxW, boxH, x, y, downW, downH, boxW, boxH, blurLabel),
			fmt.Sprintf("%s%soverlay=%d:%d:enable='between(t\\,%.3f\\,%.3f)'%s", baseLabel, blurLabel, x, y, startS, endS, outLabel),
		}, outLabel
	}
	return []string{
		prev + "split" + srcLabel + baseLabel,
		fmt.Sprintf("%scrop=%d:%d:%d:%d,boxblur=%d%s", srcLabel, boxW, boxH, x, y, radius, blurLabel),
		fmt.Sprintf("%s%soverlay=%d:%d:enable='between(t\\,%.3f\\,%.3f)'%s", baseLabel, blurLabel, x, y, startS, endS, outLabel),
	}, outLabel
}

// drawBoxFilter renders a rectangle or highlight shape via FFmpeg drawbox.
// Position derives from the clip transform (offsets from canvas center) and
// scale multiplies the shape dimensions, matching the preview. The static
// transform opacity folds into the box color's alpha.
func drawBoxFilter(clip TimelineClip, shape TimelineShape, width, height int) string {
	tr := parseClipTransform(clip.Transform)
	boxW := shape.Width
	boxH := shape.Height
	if boxW <= 0 || boxH <= 0 {
		return ""
	}
	if tr.scale != 1 {
		boxW = maxInt(2, int(float64(boxW)*tr.scale+0.5))
		boxH = maxInt(2, int(float64(boxH)*tr.scale+0.5))
	}
	x := (width-boxW)/2 + int(tr.x)
	y := (height-boxH)/2 + int(tr.y)
	startS := float64(clip.StartMS) / 1000.0
	endS := float64(clip.StartMS+clip.DurationMS) / 1000.0
	opacity := clampFloat(tr.opacity, 0, 1)
	if opacity <= 0 {
		return ""
	}
	enable := fmt.Sprintf("enable='between(t\\,%.3f\\,%.3f)'", startS, endS)
	switch shape.Kind {
	case ShapeKindHighlight:
		color := ffmpegColor(shape.Fill, "0xFACC15")
		return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@%.3f:t=fill:%s", x, y, boxW, boxH, color, opacity, enable)
	case ShapeKindRectangle, ShapeKindRoundedRectangle:
		// Rounded rectangles export with square corners — drawbox cannot
		// round; the capability matrix reports this as partial.
		color := ffmpegColor(shape.Stroke, "0xF59E0B")
		thickness := int(shape.StrokeWidth + 0.5)
		if thickness <= 0 {
			thickness = 4
		}
		return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@%.3f:t=%d:%s", x, y, boxW, boxH, color, opacity, thickness, enable)
	case ShapeKindLabel:
		// Label callouts export as a filled box; their text renders through
		// the regular drawtext pass that follows the shape in the chain.
		color := ffmpegColor(shape.Fill, "0x1E293B")
		return fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=%s@%.3f:t=fill:%s", x, y, boxW, boxH, color, opacity, enable)
	default:
		return ""
	}
}

func drawTextFilter(clip TimelineClip, text TimelineText, width, height int) string {
	fontSize := text.FontSize
	if fontSize <= 0 {
		fontSize = maxInt(24, height/18)
	}
	// The preview scales text layers by transform.scale; match it at export.
	if scale, ok := numericTransform(clip.Transform, "scale"); ok && scale > 0 && scale != 1 {
		fontSize = int(float64(fontSize)*clampFloat(scale, 0.05, 4) + 0.5)
	}
	if fontSize < 8 {
		fontSize = 8
	}
	if fontSize > 240 {
		fontSize = 240
	}
	color := ffmpegColor(text.Color, "white")
	x := "(w-text_w)/2"
	y := "(h-text_h)/2"
	if xOffset, ok := numericTransform(clip.Transform, "x"); ok && xOffset != 0 {
		x = fmt.Sprintf("(w-text_w)/2%+.0f", xOffset)
	}
	if yOffset, ok := numericTransform(clip.Transform, "y"); ok && yOffset != 0 {
		y = fmt.Sprintf("(h-text_h)/2%+.0f", yOffset)
	}
	start := float64(clip.StartMS) / 1000.0
	end := float64(clip.StartMS+clip.DurationMS) / 1000.0
	parts := []string{
		"drawtext=text='" + escapeDrawText(text.Text) + "'",
		"fontcolor=" + color,
		fmt.Sprintf("fontsize=%d", fontSize),
		"x=" + x,
		"y=" + y,
		fmt.Sprintf("enable='between(t\\,%.3f\\,%.3f)'", start, end),
	}
	if alpha := drawTextAlphaExpr(clip, start, end); alpha != "" {
		parts = append(parts, "alpha='"+alpha+"'")
	}
	if family := strings.TrimSpace(text.FontFamily); family != "" {
		// Fontconfig resolves the closest installed match, so an unknown
		// family degrades to a fallback font instead of failing the render.
		parts = append(parts, "font='"+escapeDrawText(family)+"'")
	}
	if text.LineHeight > 0 && text.LineHeight != 1 {
		spacing := int(clampFloat((text.LineHeight-1)*float64(fontSize), float64(-fontSize), float64(3*fontSize)))
		if spacing != 0 {
			parts = append(parts, fmt.Sprintf("line_spacing=%d", spacing))
		}
	}
	if text.Background != "" {
		parts = append(parts, "box=1", "boxcolor="+ffmpegColor(text.Background, "black")+"@0.55", "boxborderw=18")
	}
	if text.Shadow {
		parts = append(parts, "shadowcolor=black@0.65", "shadowx=2", "shadowy=2")
	}
	if text.Stroke != "" {
		borderWidth := 2.0
		if text.StrokeWidth > 0 {
			borderWidth = clampFloat(text.StrokeWidth, 1, 20)
		}
		parts = append(parts, fmt.Sprintf("borderw=%.0f", borderWidth), "bordercolor="+ffmpegColor(text.Stroke, "black"))
	}
	return strings.Join(parts, ":")
}

// drawTextAlphaExpr builds a drawtext alpha expression applying the clip's
// static opacity and fade in/out (matching the preview), or "" when the text
// is fully opaque with no fades. Commas are escaped for the filter graph.
func drawTextAlphaExpr(clip TimelineClip, startS, endS float64) string {
	opacity := 1.0
	if v, ok := numericTransform(clip.Transform, "opacity"); ok {
		opacity = clampFloat(v, 0, 1)
	}
	fadeInS, fadeOutS := clipFadeSeconds(clip)
	if opacity >= 1 && fadeInS <= 0 && fadeOutS <= 0 {
		return ""
	}
	var fade string
	switch {
	case fadeInS > 0 && fadeOutS > 0:
		fade = fmt.Sprintf("clip(min((t-%.3f)/%.3f,(%.3f-t)/%.3f),0,1)", startS, fadeInS, endS, fadeOutS)
	case fadeInS > 0:
		fade = fmt.Sprintf("clip((t-%.3f)/%.3f,0,1)", startS, fadeInS)
	case fadeOutS > 0:
		fade = fmt.Sprintf("clip((%.3f-t)/%.3f,0,1)", endS, fadeOutS)
	}
	expr := fmt.Sprintf("%.3f", opacity)
	if fade != "" {
		if opacity < 1 {
			expr = fmt.Sprintf("%.3f*%s", opacity, fade)
		} else {
			expr = fade
		}
	}
	return strings.ReplaceAll(expr, ",", "\\,")
}

// clipRenderTransform is the subset of clip transform values the renderer honors.
type clipRenderTransform struct {
	x, y                                     float64
	scale                                    float64
	rotation                                 float64
	opacity                                  float64
	cropTop, cropRight, cropBottom, cropLeft float64
	hasCrop                                  bool
}

// parseClipTransform extracts renderable transform values with safe defaults.
// Crop values are fractions of the source frame (0–0.95 per edge).
func parseClipTransform(transform map[string]any) clipRenderTransform {
	tr := clipRenderTransform{scale: 1, opacity: 1}
	if transform == nil {
		return tr
	}
	if v, ok := numericTransform(transform, "x"); ok {
		tr.x = v
	}
	if v, ok := numericTransform(transform, "y"); ok {
		tr.y = v
	}
	if v, ok := numericTransform(transform, "rotation"); ok {
		tr.rotation = math.Mod(v, 360)
	}
	if v, ok := numericTransform(transform, "scale"); ok && v > 0 {
		tr.scale = clampFloat(v, 0.05, 4)
	}
	if v, ok := numericTransform(transform, "opacity"); ok {
		tr.opacity = clampFloat(v, 0, 1)
	}
	if crop, ok := transform["crop"].(map[string]any); ok {
		tr.cropTop = clampFloat(numericOrZero(crop, "top"), 0, 0.95)
		tr.cropRight = clampFloat(numericOrZero(crop, "right"), 0, 0.95)
		tr.cropBottom = clampFloat(numericOrZero(crop, "bottom"), 0, 0.95)
		tr.cropLeft = clampFloat(numericOrZero(crop, "left"), 0, 0.95)
		if tr.cropTop+tr.cropBottom < 0.95 && tr.cropLeft+tr.cropRight < 0.95 &&
			(tr.cropTop > 0 || tr.cropRight > 0 || tr.cropBottom > 0 || tr.cropLeft > 0) {
			tr.hasCrop = true
		}
	}
	return tr
}

func numericOrZero(values map[string]any, key string) float64 {
	v, _ := numericTransform(values, key)
	return v
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// slideTransitionExpr builds overlay x/y offset expressions for a clip's
// first slide transition: the clip enters from `direction` over the
// transition duration and exits toward the opposite edge (continuing the
// motion), mirroring how fade-family transitions apply to both clip edges.
// Returns ("", "") when the clip has no slide transition.
func slideTransitionExpr(clip TimelineClip, startS, endS float64) (xOff, yOff string) {
	for _, transition := range clip.Transitions {
		if !strings.EqualFold(strings.TrimSpace(transition.Type), TransitionTypeSlide) || transition.DurationMS <= 0 {
			continue
		}
		d := math.Min(float64(transition.DurationMS)/1000.0, (endS-startS)/2)
		if d <= 0 {
			return "", ""
		}
		// 0→1 over the first/last d seconds of the clip.
		inProg := fmt.Sprintf("min(max((t-%.3f)/%.3f\\,0)\\,1)", startS, d)
		outProg := fmt.Sprintf("min(max((t-%.3f)/%.3f\\,0)\\,1)", endS-d, d)
		switch strings.ToLower(strings.TrimSpace(transition.Direction)) {
		case "right":
			xOff = fmt.Sprintf("W*(1-%s)-W*%s", inProg, outProg)
		case "up":
			yOff = fmt.Sprintf("-H*(1-%s)+H*%s", inProg, outProg)
		case "down":
			yOff = fmt.Sprintf("H*(1-%s)-H*%s", inProg, outProg)
		default: // left
			xOff = fmt.Sprintf("-W*(1-%s)+W*%s", inProg, outProg)
		}
		return xOff, yOff
	}
	return "", ""
}

// atempoFilters returns a product of FFmpeg atempo stages for the requested
// rate. Factoring through 0.5–2.0 works across older FFmpeg builds and keeps
// audio pitch-preserving while matching the video setpts retime.
func atempoFilters(rate float64) []string {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil
	}
	var filters []string
	for rate < 0.5-1e-9 {
		filters = append(filters, "atempo=0.500000")
		rate /= 0.5
	}
	for rate > 2+1e-9 {
		filters = append(filters, "atempo=2.000000")
		rate /= 2
	}
	if math.Abs(rate-1) > 1e-9 {
		filters = append(filters, fmt.Sprintf("atempo=%.6f", rate))
	}
	return filters
}

// clipFadeSeconds combines explicit fade_in/fade_out with fade-style
// transitions (fade, crossfade, dip_to_black), each capped at half the clip
// duration. Slide renders as animated overlay position; wipe/zoom are not
// rendered.
func clipFadeSeconds(clip TimelineClip) (fadeIn, fadeOut float64) {
	fadeInMS := clip.FadeInMS
	fadeOutMS := clip.FadeOutMS
	for _, transition := range clip.Transitions {
		switch strings.ToLower(strings.TrimSpace(transition.Type)) {
		case "fade", "crossfade", "dip_to_black":
			if transition.DurationMS > fadeInMS {
				fadeInMS = transition.DurationMS
			}
			if transition.DurationMS > fadeOutMS {
				fadeOutMS = transition.DurationMS
			}
		}
	}
	half := clip.DurationMS / 2
	if fadeInMS > half {
		fadeInMS = half
	}
	if fadeOutMS > half {
		fadeOutMS = half
	}
	return float64(fadeInMS) / 1000.0, float64(fadeOutMS) / 1000.0
}

// effectFilters maps enabled clip effects onto FFmpeg filters. Unsupported
// effect types are skipped (see FFmpegRendererCapabilities).
func effectFilters(effects []TimelineEffect) []string {
	var filters []string
	for _, effect := range effects {
		if !effect.Enabled {
			continue
		}
		amount, hasAmount := numericTransform(effect.Params, "amount")
		switch strings.ToLower(strings.TrimSpace(effect.Type)) {
		case "brightness":
			if !hasAmount {
				amount = 1
			}
			filters = append(filters, fmt.Sprintf("eq=brightness=%.3f", clampFloat(amount-1, -1, 1)))
		case "contrast":
			if !hasAmount {
				amount = 1
			}
			filters = append(filters, fmt.Sprintf("eq=contrast=%.3f", clampFloat(amount, 0, 3)))
		case "saturation":
			if !hasAmount {
				amount = 1
			}
			filters = append(filters, fmt.Sprintf("eq=saturation=%.3f", clampFloat(amount, 0, 3)))
		case "grayscale":
			filters = append(filters, "hue=s=0")
		case "blur":
			if !hasAmount || amount <= 0 {
				amount = 2
			}
			filters = append(filters, fmt.Sprintf("boxblur=%.0f", clampFloat(amount, 1, 30)))
		case "sharpen":
			if !hasAmount || amount <= 0 {
				amount = 1
			}
			filters = append(filters, fmt.Sprintf("unsharp=5:5:%.2f", clampFloat(amount, 0, 3)))
		case "vignette":
			if !hasAmount {
				amount = 0.4
			}
			if strength := clampFloat(amount, 0, 1); strength > 0 {
				filters = append(filters, fmt.Sprintf("vignette=a=%.4f", strength*math.Pi/2))
			}
		case "chroma_key":
			// Runs before format=rgba in the chain, so the keyed-out region
			// stays transparent through the overlay composite.
			color := "0x00FF00"
			if v, ok := effect.Params["color"].(string); ok && strings.TrimSpace(v) != "" {
				color = ffmpegColor(v, "0x00FF00")
			}
			similarity, ok := numericTransform(effect.Params, "similarity")
			if !ok || similarity <= 0 {
				similarity = 0.3
			}
			blend := numericOrZero(effect.Params, "blend")
			filters = append(filters, fmt.Sprintf("chromakey=%s:%.3f:%.3f",
				color, clampFloat(similarity, 0.01, 1), clampFloat(blend, 0, 0.5)))
		}
	}
	return filters
}

// positionKeyframeExpr builds a piecewise-linear FFmpeg time expression for a
// keyframed position property. Keyframe time_ms is clip-relative; clipStartS
// converts segment boundaries to output-timeline seconds (overlay expressions
// evaluate `t` in output time). The value holds flat before the first and
// after the last keyframe. Easing curves are approximated linearly at export.
// Returns "" when the property has no keyframes.
func positionKeyframeExpr(keyframes []TimelineKeyframe, property string, clipStartS float64) string {
	var points []TimelineKeyframe
	for _, keyframe := range keyframes {
		if strings.EqualFold(strings.TrimSpace(keyframe.Property), property) {
			points = append(points, keyframe)
		}
	}
	if len(points) == 0 {
		return ""
	}
	sort.Slice(points, func(i, j int) bool { return points[i].TimeMS < points[j].TimeMS })
	if len(points) == 1 {
		return fmt.Sprintf("%.3f", points[0].Value)
	}
	expr := fmt.Sprintf("%.3f", points[len(points)-1].Value)
	for i := len(points) - 1; i >= 1; i-- {
		prev, next := points[i-1], points[i]
		t0 := clipStartS + float64(prev.TimeMS)/1000.0
		t1 := clipStartS + float64(next.TimeMS)/1000.0
		span := t1 - t0
		var segment string
		if span <= 0 {
			segment = fmt.Sprintf("%.3f", next.Value)
		} else {
			segment = fmt.Sprintf("(%.3f+(%.3f)*(t-%.3f)/%.3f)", prev.Value, next.Value-prev.Value, t0, span)
		}
		expr = fmt.Sprintf("if(lt(t,%.3f),%s,%s)", t1, segment, expr)
	}
	expr = fmt.Sprintf("if(lt(t,%.3f),%.3f,%s)", clipStartS+float64(points[0].TimeMS)/1000.0, points[0].Value, expr)
	return strings.ReplaceAll(expr, ",", "\\,")
}

func numericTransform(transform map[string]any, key string) (float64, bool) {
	if transform == nil {
		return 0, false
	}
	switch value := transform[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		f, err := value.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func ffmpegColor(value, fallback string) string {
	value = strings.TrimSpace(value)
	if len(value) == 7 && value[0] == '#' && isHex(value[1:]) {
		return "0x" + value[1:]
	}
	if len(value) == 6 && isHex(value) {
		return "0x" + value
	}
	if value == "" {
		return fallback
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, value)
	if strings.TrimSpace(cleaned) == "" {
		return fallback
	}
	return cleaned
}

func isHex(value string) bool {
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return value != ""
}

func escapeDrawText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		":", "\\:",
		"'", "\\'",
		"%", "\\%",
		"[", "\\[",
		"]", "\\]",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
		"\r", "\\n",
	)
	return replacer.Replace(value)
}

func countTextClips(doc TimelineDocument) int {
	count := 0
	for _, track := range doc.Tracks {
		for _, clip := range track.Clips {
			if clip.Text != nil && strings.TrimSpace(clip.Text.Text) != "" {
				count++
			}
		}
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
