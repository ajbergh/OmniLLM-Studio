package video

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const ParityRegionPolicyVersionV1 = 1

// ParityRegionPolicyV1 assigns zero-tolerance structural regions to explicit
// output-frame identities. It is deliberately frame-index based: parity sample
// labels are diagnostic text and may be sanitized for filenames, while integer
// output-frame identity is the canonical render address.
type ParityRegionPolicyV1 struct {
	Version int                           `json:"version"`
	Canvas  ParityRegionPolicyCanvas      `json:"canvas"`
	Frames  []ParityRegionPolicyFrameV1   `json:"frames"`
}

type ParityRegionPolicyCanvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ParityRegionPolicyFrameV1 struct {
	FrameIndex int64          `json:"frame_index"`
	Regions    []ParityRegion `json:"regions"`
}

type ParityRegionPolicyEvidence struct {
	Version          int    `json:"version"`
	CanvasWidth      int    `json:"canvas_width"`
	CanvasHeight     int    `json:"canvas_height"`
	ConfiguredFrames int    `json:"configured_frames"`
	SHA256           string `json:"sha256"`
}

// ParseParityRegionPolicyV1 validates one strict policy and returns canonical
// evidence for the exact bytes supplied to the parity run.
func ParseParityRegionPolicyV1(data []byte) (ParityRegionPolicyV1, ParityRegionPolicyEvidence, error) {
	var policy ParityRegionPolicyV1
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return ParityRegionPolicyV1{}, ParityRegionPolicyEvidence{}, fmt.Errorf("decode structural-region policy: %w", err)
	}
	if err := ValidateParityRegionPolicyV1(policy); err != nil {
		return ParityRegionPolicyV1{}, ParityRegionPolicyEvidence{}, err
	}
	sum := sha256.Sum256(data)
	return policy, ParityRegionPolicyEvidence{
		Version:          policy.Version,
		CanvasWidth:      policy.Canvas.Width,
		CanvasHeight:     policy.Canvas.Height,
		ConfiguredFrames: len(policy.Frames),
		SHA256:           hex.EncodeToString(sum[:]),
	}, nil
}

// ValidateParityRegionPolicyV1 rejects ambiguous, empty, or out-of-canvas
// structural regions. Policy entries are unique by canonical frame identity.
func ValidateParityRegionPolicyV1(policy ParityRegionPolicyV1) error {
	if policy.Version != ParityRegionPolicyVersionV1 {
		return fmt.Errorf("structural-region policy version=%d, want %d", policy.Version, ParityRegionPolicyVersionV1)
	}
	if policy.Canvas.Width <= 0 || policy.Canvas.Height <= 0 {
		return fmt.Errorf("structural-region policy requires positive canvas dimensions")
	}
	if len(policy.Frames) == 0 {
		return fmt.Errorf("structural-region policy requires at least one configured frame")
	}
	seenFrames := make(map[int64]struct{}, len(policy.Frames))
	for frameOrder, frame := range policy.Frames {
		if frame.FrameIndex < 0 {
			return fmt.Errorf("structural-region policy frame at order %d has negative frame_index", frameOrder)
		}
		if _, exists := seenFrames[frame.FrameIndex]; exists {
			return fmt.Errorf("structural-region policy has duplicate frame_index %d", frame.FrameIndex)
		}
		seenFrames[frame.FrameIndex] = struct{}{}
		if len(frame.Regions) == 0 {
			return fmt.Errorf("structural-region policy frame %d has no regions", frame.FrameIndex)
		}
		seenNames := make(map[string]struct{}, len(frame.Regions))
		for regionOrder, region := range frame.Regions {
			name := strings.TrimSpace(region.Name)
			if name == "" {
				return fmt.Errorf("structural-region policy frame %d region at order %d has empty name", frame.FrameIndex, regionOrder)
			}
			if _, exists := seenNames[name]; exists {
				return fmt.Errorf("structural-region policy frame %d has duplicate region name %q", frame.FrameIndex, name)
			}
			seenNames[name] = struct{}{}
			bounds := region.Bounds
			if bounds.MinX < 0 || bounds.MinY < 0 || bounds.MaxX <= bounds.MinX || bounds.MaxY <= bounds.MinY {
				return fmt.Errorf("structural-region policy frame %d region %q has invalid bounds %+v", frame.FrameIndex, name, bounds)
			}
			if bounds.MaxX > policy.Canvas.Width || bounds.MaxY > policy.Canvas.Height {
				return fmt.Errorf("structural-region policy frame %d region %q exceeds %dx%d canvas", frame.FrameIndex, name, policy.Canvas.Width, policy.Canvas.Height)
			}
		}
	}
	return nil
}

// RegionsForParityFrame returns a copy in stable authored region order. Frames
// omitted by policy intentionally have no zero-tolerance structural regions.
func RegionsForParityFrame(policy ParityRegionPolicyV1, frameIndex int64) []ParityRegion {
	position := sort.Search(len(policy.Frames), func(i int) bool {
		return policy.Frames[i].FrameIndex >= frameIndex
	})
	if position < len(policy.Frames) && policy.Frames[position].FrameIndex == frameIndex {
		return cloneParityRegions(policy.Frames[position].Regions)
	}
	// Policy validation does not require authored frame entries to be sorted.
	for _, frame := range policy.Frames {
		if frame.FrameIndex == frameIndex {
			return cloneParityRegions(frame.Regions)
		}
	}
	return nil
}

func cloneParityRegions(regions []ParityRegion) []ParityRegion {
	if len(regions) == 0 {
		return nil
	}
	out := make([]ParityRegion, len(regions))
	copy(out, regions)
	return out
}
