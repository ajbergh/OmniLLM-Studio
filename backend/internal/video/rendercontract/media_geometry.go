package rendercontract

import (
	"fmt"
	"math"
	"strings"
)

const MediaGeometryContractV1 = "media-geometry-v1"

// EvaluatedMediaGeometry is the renderer-independent mapping from a source
// content box into the output canvas. SourceBounds describes the uncropped
// source coordinate system, VisibleSourceBounds applies mask_source_crop before
// fitting, PaintedBounds is the centered destination rectangle produced by
// media_fit, and ClipBounds applies transform.crop to the output canvas box.
type EvaluatedMediaGeometry struct {
	ContractVersion    string                  `json:"contract_version"`
	Fit                string                  `json:"fit"`
	ViewportBounds     TimelineV2ContentBounds `json:"viewport_bounds"`
	SourceBounds       TimelineV2ContentBounds `json:"source_bounds"`
	VisibleSourceBounds TimelineV2ContentBounds `json:"visible_source_bounds"`
	PaintedBounds      TimelineV2ContentBounds `json:"painted_bounds"`
	ClipBounds         TimelineV2ContentBounds `json:"clip_bounds"`
	ScaleX             float64                 `json:"scale_x"`
	ScaleY             float64                 `json:"scale_y"`
}

// EvaluateMediaGeometry resolves canonical contain/cover/fill/none placement.
// Asset-backed media must provide explicit content bounds; source aspect ratio
// is semantic input and must not be guessed from the output canvas.
func EvaluateMediaGeometry(canvas TimelineV2Canvas, clip TimelineV2Clip) (EvaluatedMediaGeometry, error) {
	if canvas.Width < 1 || canvas.Height < 1 {
		return EvaluatedMediaGeometry{}, fmt.Errorf("media geometry requires a positive canvas")
	}
	if clip.ContentBounds == nil {
		return EvaluatedMediaGeometry{}, fmt.Errorf("media geometry requires explicit content_bounds")
	}
	source := *clip.ContentBounds
	if err := validateGeometryBounds(source); err != nil {
		return EvaluatedMediaGeometry{}, fmt.Errorf("content_bounds: %w", err)
	}
	fit := strings.ToLower(strings.TrimSpace(clip.MediaFit))
	if fit == "" {
		fit = "contain"
	}
	if fit != "contain" && fit != "cover" && fit != "fill" && fit != "none" {
		return EvaluatedMediaGeometry{}, fmt.Errorf("unsupported media_fit %q", clip.MediaFit)
	}
	visible := source
	if clip.MaskSourceCrop != nil {
		var err error
		visible, err = cropBounds(source, *clip.MaskSourceCrop)
		if err != nil {
			return EvaluatedMediaGeometry{}, fmt.Errorf("mask_source_crop: %w", err)
		}
	}
	viewport := TimelineV2ContentBounds{X: 0, Y: 0, Width: float64(canvas.Width), Height: float64(canvas.Height)}
	sx, sy := 1.0, 1.0
	switch fit {
	case "contain":
		scale := math.Min(viewport.Width/visible.Width, viewport.Height/visible.Height)
		sx, sy = scale, scale
	case "cover":
		scale := math.Max(viewport.Width/visible.Width, viewport.Height/visible.Height)
		sx, sy = scale, scale
	case "fill":
		sx, sy = viewport.Width/visible.Width, viewport.Height/visible.Height
	case "none":
	}
	paintedWidth := visible.Width * sx
	paintedHeight := visible.Height * sy
	painted := TimelineV2ContentBounds{
		X:      (viewport.Width - paintedWidth) / 2,
		Y:      (viewport.Height - paintedHeight) / 2,
		Width:  paintedWidth,
		Height: paintedHeight,
	}
	clipBounds := viewport
	if clip.Transform != nil && clip.Transform.Crop != nil {
		var err error
		clipBounds, err = cropBounds(viewport, *clip.Transform.Crop)
		if err != nil {
			return EvaluatedMediaGeometry{}, fmt.Errorf("transform.crop: %w", err)
		}
	}
	return EvaluatedMediaGeometry{
		ContractVersion: MediaGeometryContractV1,
		Fit: fit, ViewportBounds: viewport, SourceBounds: source,
		VisibleSourceBounds: visible, PaintedBounds: painted, ClipBounds: clipBounds,
		ScaleX: sx, ScaleY: sy,
	}, nil
}

func validateGeometryBounds(bounds TimelineV2ContentBounds) error {
	values := []float64{bounds.X, bounds.Y, bounds.Width, bounds.Height}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("bounds must be finite")
		}
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return fmt.Errorf("width and height must be positive")
	}
	return nil
}

func cropBounds(bounds TimelineV2ContentBounds, crop TimelineV2Crop) (TimelineV2ContentBounds, error) {
	values := []float64{crop.Top, crop.Right, crop.Bottom, crop.Left}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return TimelineV2ContentBounds{}, fmt.Errorf("crop edges must be finite and between 0 and 1")
		}
	}
	if crop.Left+crop.Right >= 1 || crop.Top+crop.Bottom >= 1 {
		return TimelineV2ContentBounds{}, fmt.Errorf("crop edges must leave a positive visible area")
	}
	return TimelineV2ContentBounds{
		X:      bounds.X + bounds.Width*crop.Left,
		Y:      bounds.Y + bounds.Height*crop.Top,
		Width:  bounds.Width * (1 - crop.Left - crop.Right),
		Height: bounds.Height * (1 - crop.Top - crop.Bottom),
	}, nil
}
