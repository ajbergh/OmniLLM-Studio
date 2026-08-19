package rendercontract

import (
	"fmt"
	"math"
)

const PerspectiveProjectionContractV1 = "perspective-projection-v1"

// EvaluatedPerspectiveProjection describes the renderer-independent homogeneous
// perspective applied after the camera-relative layer model matrix. Distance is
// expressed in canvas pixels. A positive per-clip perspective overrides the
// active camera's FOV-derived distance; zero or an omitted clip perspective
// inherits the camera distance.
type EvaluatedPerspectiveProjection struct {
	ContractVersion string  `json:"contract_version"`
	Distance        float64 `json:"distance"`
	Source          string  `json:"source"`
	OriginW         float64 `json:"origin_w"`
	Matrix          Matrix4 `json:"matrix"`
}

// EvaluatePerspectiveProjection resolves the canonical CSS-style perspective
// matrix for a camera-relative layer transform. Matrix4 uses row-major storage
// with column vectors, so index 14 is the homogeneous -1/d coefficient.
func EvaluatePerspectiveProjection(camera EvaluatedCamera, view EvaluatedTransform) (EvaluatedPerspectiveProjection, error) {
	distance := camera.PerspectiveDistance
	source := "camera"
	if view.Perspective != nil && *view.Perspective != 0 {
		distance = *view.Perspective
		source = "clip"
	}
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance <= 0 {
		return EvaluatedPerspectiveProjection{}, fmt.Errorf("perspective distance must be finite and positive")
	}
	if math.IsNaN(view.Z) || math.IsInf(view.Z, 0) {
		return EvaluatedPerspectiveProjection{}, fmt.Errorf("camera-relative z must be finite")
	}
	matrix := identityMatrix()
	matrix[14] = -1 / distance
	return EvaluatedPerspectiveProjection{
		ContractVersion: PerspectiveProjectionContractV1,
		Distance:        distance,
		Source:          source,
		OriginW:         1 - view.Z/distance,
		Matrix:          matrix,
	}, nil
}
