package video

import (
	"context"
	"encoding/json"
	"fmt"
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
	inputIdx int
	clip     TimelineClip
	filePath string
	isVideo  bool
	isImage  bool
	isAudio  bool
}

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
				if rc.isAudio {
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

		filterStr, videoLabel, audioLabel := buildFilterComplex(req.Timeline, resolved, width, height)
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
	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg render failed: %w: %s", err, responseSnippet(output))
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
			"renderer":      "ffmpeg",
			"format":        format,
			"quality":       req.Settings.Quality,
			"include_audio": req.Settings.IncludeAudio,
			"text_clips":    countTextClips(req.Timeline),
		},
	}, nil
}

func renderDimensions(req RenderRequest) (int, int) {
	resolution := strings.ToLower(strings.TrimSpace(req.Settings.Resolution))
	if resolution == "" || resolution == "project" {
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
			args = append(args, "-c:a", "libopus")
		}
	default:
		crf := "23"
		if settings.Quality == "high" {
			crf = "18"
		} else if settings.Quality == "draft" {
			crf = "30"
		}
		args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", crf, "-pix_fmt", "yuv420p", "-movflags", "+faststart")
		if settings.IncludeAudio {
			args = append(args, "-c:a", "aac", "-b:a", "128k")
		}
	}
	return args
}

// resolveMediaClips iterates the timeline tracks and returns all clips that
// reference a known asset with a valid file on disk, in ascending start-time order.
func resolveMediaClips(req RenderRequest) []resolvedClip {
	if req.AttachmentsDir == "" || len(req.Assets) == 0 {
		return nil
	}
	var result []resolvedClip
	for _, track := range req.Timeline.Tracks {
		if !track.Visible || track.Muted {
			continue
		}
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
				clip:     clip,
				filePath: fullPath,
				isVideo:  strings.HasPrefix(mime, "video/"),
				isImage:  strings.HasPrefix(mime, "image/"),
				isAudio:  strings.HasPrefix(mime, "audio/"),
			}
			if rc.isVideo || rc.isImage || rc.isAudio {
				result = append(result, rc)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].clip.StartMS < result[j].clip.StartMS
	})
	return result
}

// buildFilterComplex constructs an FFmpeg -filter_complex expression that composites
// all resolved media clips onto a lavfi color background (input 0).
// It returns the filter_complex string, the final video stream label, and the final
// audio stream label (empty if no audio clips).
func buildFilterComplex(doc TimelineDocument, clips []resolvedClip, width, height int) (filterStr, videoLabel, audioLabel string) {
	var parts []string

	// ── Video / image overlays ──────────────────────────────────────────────
	prevV := "[base_v]"
	parts = append(parts, "[0:v]setpts=PTS-STARTPTS[base_v]")

	scaleChain := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
		width, height, width, height)

	for _, rc := range clips {
		if !rc.isVideo && !rc.isImage {
			continue
		}
		cLabel := fmt.Sprintf("[c%d_v]", rc.inputIdx)
		ovLabel := fmt.Sprintf("[ov%d_v]", rc.inputIdx)
		startS := float64(rc.clip.StartMS) / 1000.0
		endS := float64(rc.clip.StartMS+rc.clip.DurationMS) / 1000.0
		trimInS := float64(rc.clip.TrimInMS) / 1000.0
		durS := float64(rc.clip.DurationMS) / 1000.0

		var srcFilter string
		if rc.isImage {
			srcFilter = fmt.Sprintf("[%d:v]loop=loop=-1:size=1:start=0,trim=duration=%.3f,setpts=PTS-STARTPTS,%s%s",
				rc.inputIdx, durS, scaleChain, cLabel)
		} else {
			srcFilter = fmt.Sprintf("[%d:v]trim=start=%.3f:duration=%.3f,setpts=PTS-STARTPTS,%s%s",
				rc.inputIdx, trimInS, durS, scaleChain, cLabel)
		}
		overlayFilter := fmt.Sprintf("%s%soverlay=enable='between(t\\,%.3f\\,%.3f)':x=0:y=0%s",
			prevV, cLabel, startS, endS, ovLabel)

		parts = append(parts, srcFilter, overlayFilter)
		prevV = ovLabel
	}

	// ── Text / caption / callout overlays ──────────────────────────────────
	textIdx := 0
	for _, track := range doc.Tracks {
		if !track.Visible || track.Muted {
			continue
		}
		if track.Type != TrackTypeText && track.Type != TrackTypeCaption && track.Type != TrackTypeCallout {
			continue
		}
		for _, clip := range track.Clips {
			if clip.Text == nil || strings.TrimSpace(clip.Text.Text) == "" {
				continue
			}
			filter := drawTextFilter(clip, *clip.Text, width, height)
			if filter == "" {
				continue
			}
			outLabel := fmt.Sprintf("[t%d_v]", textIdx)
			parts = append(parts, prevV+filter+outLabel)
			prevV = outLabel
			textIdx++
		}
	}
	videoLabel = prevV

	// ── Audio clips ─────────────────────────────────────────────────────────
	var aLabels []string
	for _, rc := range clips {
		if !rc.isAudio {
			continue
		}
		aLabel := fmt.Sprintf("[a%d]", rc.inputIdx)
		trimInS := float64(rc.clip.TrimInMS) / 1000.0
		durS := float64(rc.clip.DurationMS) / 1000.0
		startMS := rc.clip.StartMS
		parts = append(parts, fmt.Sprintf("[%d:a]atrim=start=%.3f:duration=%.3f,asetpts=PTS-STARTPTS,adelay=%d|%d%s",
			rc.inputIdx, trimInS, durS, startMS, startMS, aLabel))
		aLabels = append(aLabels, aLabel)
	}
	if len(aLabels) > 1 {
		audioLabel = "[final_audio]"
		parts = append(parts, strings.Join(aLabels, "")+
			fmt.Sprintf("amix=inputs=%d:normalize=0:dropout_transition=0[final_audio]", len(aLabels)))
	} else if len(aLabels) == 1 {
		audioLabel = aLabels[0]
	}

	return strings.Join(parts, ";"), videoLabel, audioLabel
}

func ffmpegVideoFilters(doc TimelineDocument, width, height int) string {
	var filters []string
	for _, track := range doc.Tracks {
		if !track.Visible || track.Muted || (track.Type != TrackTypeText && track.Type != TrackTypeCaption && track.Type != TrackTypeCallout) {
			continue
		}
		for _, clip := range track.Clips {
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

func drawTextFilter(clip TimelineClip, text TimelineText, width, height int) string {
	fontSize := text.FontSize
	if fontSize <= 0 {
		fontSize = maxInt(24, height/18)
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
	if text.Background != "" {
		parts = append(parts, "box=1", "boxcolor="+ffmpegColor(text.Background, "black")+"@0.55", "boxborderw=18")
	}
	if text.Shadow {
		parts = append(parts, "shadowcolor=black@0.65", "shadowx=2", "shadowy=2")
	}
	if text.Stroke != "" {
		parts = append(parts, "borderw=2", "bordercolor="+ffmpegColor(text.Stroke, "black"))
	}
	return strings.Join(parts, ":")
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
		if track.Type != TrackTypeText && track.Type != TrackTypeCaption && track.Type != TrackTypeCallout {
			continue
		}
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
