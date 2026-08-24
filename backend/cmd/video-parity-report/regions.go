package main

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
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
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest parityRegionManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode parity region manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parity region manifest must contain exactly one JSON object")
		}
		return nil, fmt.Errorf("decode parity region manifest trailing data: %w", err)
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

// validateParityRegionsForPair prevents exact-region comparisons from clipping
// silently at decoded image bounds. A clipped or fully out-of-frame rectangle
// could otherwise report Exact=true while comparing fewer (or zero) pixels.
func validateParityRegionsForPair(frameIndex int64, regions []video.ParityRegion, preview, rendered image.Image) error {
	previewBounds, renderedBounds := preview.Bounds(), rendered.Bounds()
	for _, region := range regions {
		bounds := region.Bounds
		if bounds.MaxX > previewBounds.Dx() || bounds.MaxY > previewBounds.Dy() ||
			bounds.MaxX > renderedBounds.Dx() || bounds.MaxY > renderedBounds.Dy() {
			return fmt.Errorf(
				"frame_index %d region %q bounds [%d,%d)-[%d,%d) exceed decoded frame dimensions preview=%dx%d rendered=%dx%d",
				frameIndex, region.Name, bounds.MinX, bounds.MinY, bounds.MaxX, bounds.MaxY,
				previewBounds.Dx(), previewBounds.Dy(), renderedBounds.Dx(), renderedBounds.Dy(),
			)
		}
	}
	return nil
}

func cloneParityRegions(regions []video.ParityRegion) []video.ParityRegion {
	if len(regions) == 0 {
		return nil
	}
	cloned := make([]video.ParityRegion, len(regions))
	copy(cloned, regions)
	return cloned
}
