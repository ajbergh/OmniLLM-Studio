package video

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type ParityEBUR128Measurement struct {
	IntegratedLUFS float64
	TruePeakDBFS   float64
}

var (
	parityIntegratedLUFSPattern = regexp.MustCompile(`(?m)^\s*I:\s+([^\s]+)\s+LUFS\s*$`)
	parityTruePeakPattern       = regexp.MustCompile(`(?m)^\s*Peak:\s+([^\s]+)\s+dBFS\s*$`)
)

// MeasureParityEBUR128 runs FFmpeg's standards-based EBU R128 scanner over
// exact-format parity PCM. Parsing is restricted to the final Summary block
// so periodic scanner log lines cannot be mistaken for the final result.
func MeasureParityEBUR128(ctx context.Context, ffmpegBinary, pcmPath string) (ParityEBUR128Measurement, error) {
	if strings.TrimSpace(ffmpegBinary) == "" {
		var err error
		ffmpegBinary, err = exec.LookPath("ffmpeg")
		if err != nil {
			return ParityEBUR128Measurement{}, fmt.Errorf("ffmpeg was not found in PATH")
		}
	}
	command := exec.CommandContext(ctx, ffmpegBinary,
		"-hide_banner", "-nostats", "-f", "s16le", "-ar", "48000", "-ac", "2", "-i", pcmPath,
		"-filter_complex", "ebur128=peak=true", "-f", "null", "-",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return ParityEBUR128Measurement{}, fmt.Errorf("measure EBU R128: %w: %s", err, responseSnippet(output))
	}
	measurement, err := parseParityEBUR128Summary(string(output))
	if err != nil {
		return ParityEBUR128Measurement{}, fmt.Errorf("parse EBU R128 summary: %w", err)
	}
	return measurement, nil
}

func parseParityEBUR128Summary(output string) (ParityEBUR128Measurement, error) {
	summaryAt := strings.LastIndex(output, "Summary:")
	if summaryAt < 0 {
		return ParityEBUR128Measurement{}, fmt.Errorf("summary was not present")
	}
	summary := output[summaryAt:]
	integratedMatch := parityIntegratedLUFSPattern.FindStringSubmatch(summary)
	peakMatch := parityTruePeakPattern.FindStringSubmatch(summary)
	if len(integratedMatch) != 2 || len(peakMatch) != 2 {
		return ParityEBUR128Measurement{}, fmt.Errorf("integrated loudness or true peak was not present")
	}
	integrated, err := parseParityDBValue(integratedMatch[1])
	if err != nil {
		return ParityEBUR128Measurement{}, fmt.Errorf("integrated loudness: %w", err)
	}
	peak, err := parseParityDBValue(peakMatch[1])
	if err != nil {
		return ParityEBUR128Measurement{}, fmt.Errorf("true peak: %w", err)
	}
	return ParityEBUR128Measurement{IntegratedLUFS: integrated, TruePeakDBFS: peak}, nil
}

func parseParityDBValue(value string) (float64, error) {
	if strings.EqualFold(strings.TrimSpace(value), "-inf") {
		return math.Inf(-1), nil
	}
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func parityDBDifference(a, b float64) float64 {
	if math.IsInf(a, 0) && math.IsInf(b, 0) && math.Signbit(a) == math.Signbit(b) {
		return 0
	}
	return math.Abs(a - b)
}

// ApplyParityEBUR128 attaches the two measurements and makes their tolerances
// part of the audio pass/fail decision.
func ApplyParityEBUR128(metric *ParityAudioMetric, preview, rendered ParityEBUR128Measurement, thresholds ParityThresholds) {
	if metric == nil {
		return
	}
	metric.EBUR128Measured = true
	metric.PreviewIntegratedLUFS = preview.IntegratedLUFS
	metric.RenderedIntegratedLUFS = rendered.IntegratedLUFS
	metric.IntegratedLUFSDifference = parityDBDifference(preview.IntegratedLUFS, rendered.IntegratedLUFS)
	metric.PreviewTruePeakDBFS = preview.TruePeakDBFS
	metric.RenderedTruePeakDBFS = rendered.TruePeakDBFS
	metric.TruePeakDBFSDifference = parityDBDifference(preview.TruePeakDBFS, rendered.TruePeakDBFS)
	metric.Pass = metric.Pass && metric.IntegratedLUFSDifference <= thresholds.MaximumAudioLUFSDifference+1e-12 &&
		metric.TruePeakDBFSDifference <= thresholds.MaximumAudioTruePeakDifference+1e-12
}
