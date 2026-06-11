package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// MediaProbe holds metadata extracted from a media file via ffprobe.
type MediaProbe struct {
	DurationMS int64
	Width      int
	Height     int
	FPS        float64
}

// ProbeMedia extracts duration/dimensions/FPS using ffprobe when available.
// Returns (nil, nil) when ffprobe is not installed or yields nothing useful —
// uploads must keep working without it, so callers treat probe data as
// best-effort enrichment.
func ProbeMedia(ctx context.Context, path string) (*MediaProbe, error) {
	binary, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, binary,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	return parseProbePayload(output)
}

func parseProbePayload(output []byte) (*MediaProbe, error) {
	var payload struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType    string `json:"codec_type"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			RFrameRate   string `json:"r_frame_rate"`
			AvgFrameRate string `json:"avg_frame_rate"`
			Duration     string `json:"duration"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	probe := &MediaProbe{}
	if seconds, err := strconv.ParseFloat(strings.TrimSpace(payload.Format.Duration), 64); err == nil && seconds > 0 {
		probe.DurationMS = int64(seconds * 1000)
	}
	for _, stream := range payload.Streams {
		if stream.CodecType != "video" {
			continue
		}
		probe.Width = stream.Width
		probe.Height = stream.Height
		probe.FPS = parseFrameRate(stream.AvgFrameRate)
		if probe.FPS == 0 {
			probe.FPS = parseFrameRate(stream.RFrameRate)
		}
		if probe.DurationMS == 0 {
			if seconds, err := strconv.ParseFloat(strings.TrimSpace(stream.Duration), 64); err == nil && seconds > 0 {
				probe.DurationMS = int64(seconds * 1000)
			}
		}
		break
	}
	if probe.DurationMS == 0 && probe.Width == 0 && probe.Height == 0 {
		return nil, nil
	}
	return probe, nil
}

// parseFrameRate parses ffprobe rational frame rates like "30000/1001" or "30/1".
func parseFrameRate(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "0/0" {
		return 0
	}
	if num, den, found := strings.Cut(value, "/"); found {
		n, errN := strconv.ParseFloat(num, 64)
		d, errD := strconv.ParseFloat(den, 64)
		if errN != nil || errD != nil || d == 0 {
			return 0
		}
		return n / d
	}
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return rate
}
