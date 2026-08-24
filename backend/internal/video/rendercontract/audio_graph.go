package rendercontract

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	AudioGraphContractV1              = "audio-graph-v1"
	AudioGraphSampleRateV1            = 48000
	AudioGraphChannelsV1              = 2
	AudioGraphChannelLayoutStereo     = "stereo"
	AudioGraphMixPolicySumNoNormalize = "sum-no-normalize"
	AudioGraphPitchPolicyPreserve     = "preserve"
	AudioGraphFadeCurveLinear         = "linear"
	AudioGraphFadeCombineMinimum      = "minimum"
	AudioGraphProgramStagePostMix     = "post-mix"
	AudioGraphProcessingNone          = "none"
	AudioGraphProcessingStemRequired  = "processed-stem-required"
)

type AudioGraphV1 struct {
	ContractVersion     string                      `json:"contract_version"`
	SampleRate          int                         `json:"sample_rate"`
	Channels            int                         `json:"channels"`
	ChannelLayout       string                      `json:"channel_layout"`
	TimelineSampleCount int64                       `json:"timeline_sample_count"`
	RangeStartSample    int64                       `json:"range_start_sample"`
	RangeEndSample      int64                       `json:"range_end_sample"`
	RangeSampleCount    int64                       `json:"range_sample_count"`
	MixPolicy           string                      `json:"mix_policy"`
	ProgramStemID       string                      `json:"program_stem_id"`
	Sources             []AudioGraphSourceV1        `json:"sources"`
	ProgramProcessing   AudioGraphProgramProcessing `json:"program_processing"`
}

type AudioGraphSourceV1 struct {
	NodeID            string                   `json:"node_id"`
	StemID            string                   `json:"stem_id"`
	TrackID           string                   `json:"track_id"`
	ClipID            string                   `json:"clip_id"`
	AssetID           string                   `json:"asset_id"`
	TrackIndex        int                      `json:"track_index"`
	ClipIndex         int                      `json:"clip_index"`
	Enabled           bool                     `json:"enabled"`
	SuppressionReason string                   `json:"suppression_reason"`
	StartSample       int64                    `json:"start_sample"`
	EndSample         int64                    `json:"end_sample"`
	OutputSampleCount int64                    `json:"output_sample_count"`
	SourceStartMS     int64                    `json:"source_start_ms"`
	SourceEndMS       int64                    `json:"source_end_ms"`
	PlaybackRate      float64                  `json:"playback_rate"`
	PitchPolicy       string                   `json:"pitch_policy"`
	SourceChannels    int                      `json:"source_channels"`
	ChannelMap        string                   `json:"channel_map"`
	BaseGain          float64                  `json:"base_gain"`
	GainMode          string                   `json:"gain_mode"`
	GainKeyframes     []AudioGraphGainKeyframe `json:"gain_keyframes"`
	FadeInSamples     int64                    `json:"fade_in_samples"`
	FadeOutSamples    int64                    `json:"fade_out_samples"`
	FadeCurve         string                   `json:"fade_curve"`
	FadeCombinePolicy string                   `json:"fade_combine_policy"`
}

type AudioGraphGainKeyframe struct {
	ID            string       `json:"id"`
	AuthoredOrder int          `json:"authored_order"`
	TimeMS        int64        `json:"time_ms"`
	Value         float64      `json:"value"`
	Easing        string       `json:"easing,omitempty"`
	Curve         *MotionCurve `json:"curve,omitempty"`
}

type AudioGraphProgramProcessing struct {
	Mode         string  `json:"mode"`
	Stage        string  `json:"stage"`
	InputStemID  string  `json:"input_stem_id"`
	OutputStemID string  `json:"output_stem_id"`
	Denoise      bool    `json:"denoise"`
	EQPreset     string  `json:"eq_preset"`
	Compressor   bool    `json:"compressor"`
	Normalize    bool    `json:"normalize"`
	TargetLUFS   float64 `json:"target_lufs"`
	Limiter      bool    `json:"limiter"`
	ChannelMode  string  `json:"channel_mode"`
}

// EvaluateAudioGraphV1 derives the canonical renderer-independent audio graph
// for one immutable render manifest. It owns selection, source timing,
// rate/pitch policy, channel mapping, gain/fade semantics, mix order, program
// processing placement, and output sample-count decisions. It deliberately
// performs no media I/O or DSP.
func EvaluateAudioGraphV1(manifest RenderManifestV1) (AudioGraphV1, error) {
	if manifest.Settings.AudioSampleRate != AudioGraphSampleRateV1 {
		return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 requires audio_sample_rate=%d", AudioGraphSampleRateV1)
	}
	if manifest.Settings.AudioChannels != AudioGraphChannelsV1 {
		return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 requires audio_channels=%d", AudioGraphChannelsV1)
	}

	doc, err := NormalizeTimelineV2EvaluationInputs(manifest.Timeline)
	if err != nil {
		return AudioGraphV1{}, err
	}
	if doc.Canvas.FPS <= 0 {
		return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 requires a positive timeline fps")
	}
	if manifest.Settings.RangeStartFrame < 0 || manifest.Settings.RangeEndFrame < manifest.Settings.RangeStartFrame {
		return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 has invalid render frame range [%d,%d)", manifest.Settings.RangeStartFrame, manifest.Settings.RangeEndFrame)
	}

	timelineSamples := millisecondsToSamplesCeil(doc.DurationMS, AudioGraphSampleRateV1)
	rangeStart := frameToSamplesFloor(manifest.Settings.RangeStartFrame, doc.Canvas.FPS, AudioGraphSampleRateV1)
	rangeEnd := frameToSamplesCeil(manifest.Settings.RangeEndFrame, doc.Canvas.FPS, AudioGraphSampleRateV1)
	if rangeStart > timelineSamples {
		rangeStart = timelineSamples
	}
	if rangeEnd > timelineSamples {
		rangeEnd = timelineSamples
	}
	if rangeEnd < rangeStart {
		rangeEnd = rangeStart
	}

	assets := make(map[string]RenderManifestAsset, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		id := strings.TrimSpace(asset.AssetID)
		if id == "" {
			return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 asset at order %d has empty asset_id", index)
		}
		if _, exists := assets[id]; exists {
			return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 has duplicate asset_id %q", id)
		}
		assets[id] = asset
	}

	anySolo := false
	for _, track := range doc.Tracks {
		if track.Solo {
			anySolo = true
			break
		}
	}

	sources := make([]AudioGraphSourceV1, 0)
	for trackIndex, track := range doc.Tracks {
		for clipIndex, clip := range track.Clips {
			assetID := strings.TrimSpace(clip.AssetID)
			if assetID == "" {
				if track.Type == "audio" || track.Type == "music" {
					return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 audio clip %q has no asset_id", clip.ID)
				}
				continue
			}
			asset, ok := assets[assetID]
			if !ok {
				return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 clip %q references missing manifest asset %q", clip.ID, assetID)
			}

			sourceChannels := 0
			if asset.Media != nil && asset.Media.Channels != nil {
				sourceChannels = *asset.Media.Channels
			}
			audioKind := asset.Kind == "audio" || asset.Kind == "music"
			if sourceChannels <= 0 {
				if audioKind {
					return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 audio asset %q has no probed audio channel count", assetID)
				}
				continue
			}
			channelMap, err := canonicalAudioChannelMap(sourceChannels)
			if err != nil {
				return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 clip %q: %w", clip.ID, err)
			}

			baseGain := 1.0
			if clip.Volume != nil {
				baseGain = *clip.Volume
			}
			if !isFinite(baseGain) || baseGain < 0 || baseGain > 2 {
				return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 clip %q volume must be finite and between 0 and 2", clip.ID)
			}
			gainKeyframes, err := canonicalAudioGainKeyframes(clip)
			if err != nil {
				return AudioGraphV1{}, err
			}
			if clip.FadeInMS < 0 || clip.FadeOutMS < 0 {
				return AudioGraphV1{}, fmt.Errorf("audio-graph-v1 clip %q fade durations cannot be negative", clip.ID)
			}

			startSample := millisecondsToSamplesFloor(clip.StartMS, AudioGraphSampleRateV1)
			endSample := millisecondsToSamplesCeil(clip.StartMS+clip.DurationMS, AudioGraphSampleRateV1)
			if endSample > timelineSamples {
				endSample = timelineSamples
			}
			if startSample > timelineSamples {
				startSample = timelineSamples
			}
			if endSample < startSample {
				endSample = startSample
			}

			enabled, reason := audioSourceEnabled(track, clip, anySolo, startSample, endSample)
			sources = append(sources, AudioGraphSourceV1{
				NodeID:            "source:" + clip.ID,
				StemID:            "clip:" + clip.ID,
				TrackID:           track.ID,
				ClipID:            clip.ID,
				AssetID:           assetID,
				TrackIndex:        trackIndex,
				ClipIndex:         clipIndex,
				Enabled:           enabled,
				SuppressionReason: reason,
				StartSample:       startSample,
				EndSample:         endSample,
				OutputSampleCount: endSample - startSample,
				SourceStartMS:     clip.TrimInMS,
				SourceEndMS:       clip.TrimOutMS,
				PlaybackRate:      *clip.PlaybackRate,
				PitchPolicy:       AudioGraphPitchPolicyPreserve,
				SourceChannels:    sourceChannels,
				ChannelMap:        channelMap,
				BaseGain:          baseGain,
				GainMode:          "automation-overrides-base",
				GainKeyframes:     gainKeyframes,
				FadeInSamples:     millisecondsToSamplesCeil(clip.FadeInMS, AudioGraphSampleRateV1),
				FadeOutSamples:    millisecondsToSamplesCeil(clip.FadeOutMS, AudioGraphSampleRateV1),
				FadeCurve:         AudioGraphFadeCurveLinear,
				FadeCombinePolicy: AudioGraphFadeCombineMinimum,
			})
		}
	}

	processing, err := canonicalAudioProgramProcessing(doc.Metadata)
	if err != nil {
		return AudioGraphV1{}, err
	}
	return AudioGraphV1{
		ContractVersion:     AudioGraphContractV1,
		SampleRate:          AudioGraphSampleRateV1,
		Channels:            AudioGraphChannelsV1,
		ChannelLayout:       AudioGraphChannelLayoutStereo,
		TimelineSampleCount: timelineSamples,
		RangeStartSample:    rangeStart,
		RangeEndSample:      rangeEnd,
		RangeSampleCount:    rangeEnd - rangeStart,
		MixPolicy:           AudioGraphMixPolicySumNoNormalize,
		ProgramStemID:       "program-mix",
		Sources:             sources,
		ProgramProcessing:   processing,
	}, nil
}

func canonicalAudioGainKeyframes(clip TimelineV2Clip) ([]AudioGraphGainKeyframe, error) {
	type ordered struct {
		keyframe TimelineV2Keyframe
		order    int
	}
	points := make([]ordered, 0)
	for order, keyframe := range clip.Keyframes {
		if strings.ToLower(strings.TrimSpace(keyframe.Property)) != "volume" {
			continue
		}
		if strings.TrimSpace(keyframe.ID) == "" {
			return nil, fmt.Errorf("audio-graph-v1 clip %q has a volume keyframe with empty id", clip.ID)
		}
		if !isFinite(keyframe.Value) || keyframe.Value < 0 || keyframe.Value > 2 {
			return nil, fmt.Errorf("audio-graph-v1 clip %q volume keyframe %q value must be finite and between 0 and 2", clip.ID, keyframe.ID)
		}
		points = append(points, ordered{keyframe: keyframe, order: order})
	}
	sort.Slice(points, func(i, j int) bool {
		if points[i].keyframe.TimeMS != points[j].keyframe.TimeMS {
			return points[i].keyframe.TimeMS < points[j].keyframe.TimeMS
		}
		return points[i].order < points[j].order
	})
	out := make([]AudioGraphGainKeyframe, 0, len(points))
	for _, point := range points {
		out = append(out, AudioGraphGainKeyframe{
			ID:            strings.TrimSpace(point.keyframe.ID),
			AuthoredOrder: point.order,
			TimeMS:        point.keyframe.TimeMS,
			Value:         point.keyframe.Value,
			Easing:        strings.ToLower(strings.TrimSpace(point.keyframe.Easing)),
			Curve:         cloneMotionCurve(point.keyframe.Curve),
		})
	}
	return out, nil
}

func cloneMotionCurve(curve *MotionCurve) *MotionCurve {
	if curve == nil {
		return nil
	}
	copied := *curve
	copied.Type = strings.ToLower(strings.TrimSpace(copied.Type))
	return &copied
}

func canonicalAudioChannelMap(sourceChannels int) (string, error) {
	switch sourceChannels {
	case 1:
		return "mono-to-stereo", nil
	case 2:
		return "stereo-passthrough", nil
	default:
		return "", fmt.Errorf("source channel count %d has no canonical v1 channel mapping", sourceChannels)
	}
}

func audioSourceEnabled(track TimelineV2Track, clip TimelineV2Clip, anySolo bool, startSample, endSample int64) (bool, string) {
	switch {
	case track.Muted:
		return false, "track-muted"
	case clip.Muted:
		return false, "clip-muted"
	case anySolo && !track.Solo:
		return false, "solo-suppressed"
	case endSample <= startSample:
		return false, "outside-timeline"
	default:
		return true, ""
	}
}

func canonicalAudioProgramProcessing(metadata Metadata) (AudioGraphProgramProcessing, error) {
	out := AudioGraphProgramProcessing{
		Mode:         AudioGraphProcessingNone,
		Stage:        AudioGraphProgramStagePostMix,
		InputStemID:  "program-mix",
		OutputStemID: "program-output",
		TargetLUFS:   -16,
		EQPreset:     "none",
		ChannelMode:  "source",
	}
	if metadata == nil {
		return out, nil
	}
	raw, exists := metadata["render_audio_processing"]
	if !exists || raw == nil {
		return out, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		if converted, okMetadata := raw.(Metadata); okMetadata {
			values = map[string]any(converted)
		} else {
			return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing must be an object")
		}
	}
	required := []string{"normalize", "target_lufs", "denoise", "eq_preset", "compressor", "limiter", "channels"}
	allowed := make(map[string]bool, len(required))
	for _, key := range required {
		allowed[key] = true
		if _, ok := values[key]; !ok {
			return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.%s is required", key)
		}
	}
	for key := range values {
		if !allowed[key] {
			return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing has unsupported field %q", key)
		}
	}
	var err error
	if out.Normalize, err = audioGraphBool(values, "normalize"); err != nil {
		return AudioGraphProgramProcessing{}, err
	}
	if out.Denoise, err = audioGraphBool(values, "denoise"); err != nil {
		return AudioGraphProgramProcessing{}, err
	}
	if out.Compressor, err = audioGraphBool(values, "compressor"); err != nil {
		return AudioGraphProgramProcessing{}, err
	}
	if out.Limiter, err = audioGraphBool(values, "limiter"); err != nil {
		return AudioGraphProgramProcessing{}, err
	}
	target, ok := effectNumericValue(values["target_lufs"])
	if !ok || !isFinite(target) || target < -30 || target > -5 {
		return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.target_lufs must be finite and between -30 and -5")
	}
	out.TargetLUFS = target
	eq, ok := values["eq_preset"].(string)
	if !ok {
		return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.eq_preset must be a string")
	}
	out.EQPreset = strings.ToLower(strings.TrimSpace(eq))
	switch out.EQPreset {
	case "none", "voice", "warm", "bright":
	default:
		return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.eq_preset %q is unsupported", out.EQPreset)
	}
	channels, ok := values["channels"].(string)
	if !ok {
		return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.channels must be a string")
	}
	out.ChannelMode = strings.ToLower(strings.TrimSpace(channels))
	switch out.ChannelMode {
	case "source", "mono", "stereo":
	default:
		return AudioGraphProgramProcessing{}, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.channels %q is unsupported", out.ChannelMode)
	}
	out.Mode = AudioGraphProcessingStemRequired
	return out, nil
}

func audioGraphBool(values map[string]any, key string) (bool, error) {
	value, ok := values[key].(bool)
	if !ok {
		return false, fmt.Errorf("audio-graph-v1 metadata.render_audio_processing.%s must be a boolean", key)
	}
	return value, nil
}

func millisecondsToSamplesFloor(ms int64, sampleRate int) int64 {
	if ms <= 0 || sampleRate <= 0 {
		return 0
	}
	return (ms * int64(sampleRate)) / 1000
}

func millisecondsToSamplesCeil(ms int64, sampleRate int) int64 {
	if ms <= 0 || sampleRate <= 0 {
		return 0
	}
	product := ms * int64(sampleRate)
	return (product + 999) / 1000
}

func frameToSamplesFloor(frame int64, fps, sampleRate int) int64 {
	if frame <= 0 || fps <= 0 || sampleRate <= 0 {
		return 0
	}
	return (frame * int64(sampleRate)) / int64(fps)
}

func frameToSamplesCeil(frame int64, fps, sampleRate int) int64 {
	if frame <= 0 || fps <= 0 || sampleRate <= 0 {
		return 0
	}
	product := frame * int64(sampleRate)
	divisor := int64(fps)
	return (product + divisor - 1) / divisor
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
