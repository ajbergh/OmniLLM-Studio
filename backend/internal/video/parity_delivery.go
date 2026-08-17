package video

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

type ParityDeliveryMetric struct {
	ExpectedFrameCount       int64   `json:"expected_frame_count"`
	ActualFrameCount         int64   `json:"actual_frame_count"`
	FrameCountMatches        bool    `json:"frame_count_matches"`
	ExpectedFPS              float64 `json:"expected_fps"`
	ActualFPS                float64 `json:"actual_fps"`
	ConstantFrameRate        bool    `json:"constant_frame_rate"`
	ExpectedStartTimeSeconds float64 `json:"expected_start_time_seconds"`
	ActualStartTimeSeconds   float64 `json:"actual_start_time_seconds"`
	ExpectedDurationSeconds  float64 `json:"expected_duration_seconds"`
	ActualDurationSeconds    float64 `json:"actual_duration_seconds"`
	TimeBase                 string  `json:"time_base,omitempty"`
	Pass                     bool    `json:"pass"`
}

type parityFFprobeDocument struct {
	Streams []struct {
		ReadFrames   string `json:"nb_read_frames"`
		Frames       string `json:"nb_frames"`
		StartTime    string `json:"start_time"`
		Duration     string `json:"duration"`
		AverageRate  string `json:"avg_frame_rate"`
		DeclaredRate string `json:"r_frame_rate"`
		TimeBase     string `json:"time_base"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ProbeParityDelivery verifies decoded delivery timing rather than trusting
// container metadata written by the renderer. count_frames forces FFprobe to
// walk the selected video stream and report the actual decoded frame count.
func ProbeParityDelivery(ctx context.Context, ffprobeBinary, mediaPath string, expectedDurationMS int64, expectedFPS float64) (ParityDeliveryMetric, error) {
	if expectedDurationMS <= 0 || expectedFPS <= 0 {
		return ParityDeliveryMetric{}, fmt.Errorf("expected duration and FPS must be positive")
	}
	if strings.TrimSpace(ffprobeBinary) == "" {
		var err error
		ffprobeBinary, err = exec.LookPath("ffprobe")
		if err != nil {
			return ParityDeliveryMetric{}, fmt.Errorf("ffprobe was not found in PATH")
		}
	}
	command := exec.CommandContext(ctx, ffprobeBinary,
		"-v", "error", "-count_frames", "-select_streams", "v:0",
		"-show_entries", "stream=nb_read_frames,nb_frames,start_time,duration,avg_frame_rate,r_frame_rate,time_base",
		"-show_entries", "format=duration", "-of", "json", mediaPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return ParityDeliveryMetric{}, fmt.Errorf("ffprobe delivery: %w: %s", err, responseSnippet(output))
	}
	var probe parityFFprobeDocument
	if err := json.Unmarshal(output, &probe); err != nil {
		return ParityDeliveryMetric{}, fmt.Errorf("decode ffprobe delivery JSON: %w", err)
	}
	if len(probe.Streams) == 0 {
		return ParityDeliveryMetric{}, fmt.Errorf("delivery has no video stream")
	}
	stream := probe.Streams[0]
	actualFrames, err := parseParityInt(parityFirstNonEmpty(stream.ReadFrames, stream.Frames))
	if err != nil {
		return ParityDeliveryMetric{}, fmt.Errorf("delivery frame count: %w", err)
	}
	actualFPS, err := parseParityRate(stream.AverageRate)
	if err != nil {
		return ParityDeliveryMetric{}, fmt.Errorf("delivery average frame rate: %w", err)
	}
	declaredFPS, _ := parseParityRate(stream.DeclaredRate)
	start, err := parseParityFloat(stream.StartTime)
	if err != nil {
		return ParityDeliveryMetric{}, fmt.Errorf("delivery start time: %w", err)
	}
	duration, err := parseParityFloat(parityFirstNonEmpty(stream.Duration, probe.Format.Duration))
	if err != nil {
		return ParityDeliveryMetric{}, fmt.Errorf("delivery duration: %w", err)
	}
	expectedDuration := float64(expectedDurationMS) / 1000
	expectedFrames := int64(math.Ceil(expectedDuration*expectedFPS - 1e-9))
	frameDuration := 1 / expectedFPS
	metric := ParityDeliveryMetric{
		ExpectedFrameCount: expectedFrames, ActualFrameCount: actualFrames, FrameCountMatches: actualFrames == expectedFrames,
		ExpectedFPS: expectedFPS, ActualFPS: actualFPS, ConstantFrameRate: math.Abs(actualFPS-expectedFPS) <= 1e-9 && math.Abs(declaredFPS-actualFPS) <= 1e-9,
		ExpectedStartTimeSeconds: 0, ActualStartTimeSeconds: start,
		ExpectedDurationSeconds: expectedDuration, ActualDurationSeconds: duration, TimeBase: stream.TimeBase,
	}
	metric.Pass = metric.FrameCountMatches && metric.ConstantFrameRate && math.Abs(metric.ActualStartTimeSeconds) <= 1e-9 && math.Abs(metric.ActualDurationSeconds-expectedDuration) <= frameDuration/2
	return metric, nil
}

func parityFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "N/A" {
			return value
		}
	}
	return ""
}

func parseParityInt(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

func parseParityFloat(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func parseParityRate(value string) (float64, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) == 1 {
		return parseParityFloat(parts[0])
	}
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid rate %q", value)
	}
	numerator, err := parseParityFloat(parts[0])
	if err != nil {
		return 0, err
	}
	denominator, err := parseParityFloat(parts[1])
	if err != nil || denominator == 0 {
		return 0, fmt.Errorf("invalid rate %q", value)
	}
	return numerator / denominator, nil
}
