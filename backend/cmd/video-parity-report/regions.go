package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

const parityRegionManifestVersion = 1

type parityRegionManifest struct {
	Version int                      `json:"version"`
	Frames  []parityRegionFrameInput `json:"frames"`
}

type parityRegionFrameInput struct {
	FrameIndex int64                `json:"frame_index"`
	Regions    []video.ParityRegion `json:"regions"`
}

// loadParityRegionManifest reads an optional versioned frame-region policy.
// Regions are keyed by canonical output frame index so report filenames and
// human-readable sample labels cannot silently change the policy binding.
func loadParityRegionManifest(path string) (map[int64][]video.ParityRegion, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest parityRegionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode parity region manifest: %w", err)
	}
	if manifest.Version != parityRegionManifestVersion {
		return nil, fmt.Errorf("parity region manifest version = %d, want %d", manifest.Version, parityRegionManifestVersion)
	}

	byFrame := make(map[int64][]video.ParityRegion, len(manifest.Frames))
	for framePosition, frame := range manifest.Frames {
		if frame.FrameIndex < 0 {
			return nil, fmt.Errorf("frames[%d].frame_index must be non-negative", framePosition)
		}
		if _, duplicate := byFrame[frame.FrameIndex]; duplicate {
			return nil, fmt.Errorf("duplicate frame_index %d in parity region manifest", frame.FrameIndex)
		}
		regions := make([]video.ParityRegion, len(frame.Regions))
		seenNames := make(map[string]struct{}, len(frame.Regions))
		for regionPosition, region := range frame.Regions {
			region.Name = strings.TrimSpace(region.Name)
			if region.Name == "" {
				return nil, fmt.Errorf("frames[%d].regions[%d].name must not be empty", framePosition, regionPosition)
			}
			if _, duplicate := seenNames[region.Name]; duplicate {
				return nil, fmt.Errorf("duplicate region name %q for frame_index %d", region.Name, frame.FrameIndex)
			}
			seenNames[region.Name] = struct{}{}
			bounds := region.Bounds
			if bounds.MinX < 0 || bounds.MinY < 0 || bounds.MaxX <= bounds.MinX || bounds.MaxY <= bounds.MinY {
				return nil, fmt.Errorf("frames[%d].regions[%d].bounds must define a positive non-negative rectangle", framePosition, regionPosition)
			}
			regions[regionPosition] = region
		}
		byFrame[frame.FrameIndex] = regions
	}
	return byFrame, nil
}

func cloneParityRegions(regions []video.ParityRegion) []video.ParityRegion {
	if len(regions) == 0 {
		return nil
	}
	cloned := make([]video.ParityRegion, len(regions))
	copy(cloned, regions)
	return cloned
}
