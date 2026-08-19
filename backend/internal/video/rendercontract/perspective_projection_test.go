package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type perspectiveProjectionFixture struct {
	Version int `json:"version"`
	Cases   []struct {
		Name             string   `json:"name"`
		CameraDistance   float64  `json:"camera_distance"`
		ClipPerspective  *float64 `json:"clip_perspective"`
		ViewZ            float64  `json:"view_z"`
		ExpectedSource   string   `json:"expected_source"`
		ExpectedDistance float64  `json:"expected_distance"`
		ExpectedOriginW  float64  `json:"expected_origin_w"`
		ExpectedMatrix   Matrix4  `json:"expected_matrix"`
	} `json:"cases"`
}

func TestEvaluatePerspectiveProjectionMatchesSharedFixture(t *testing.T) {
	fixture := loadPerspectiveProjectionFixture(t)
	for _, sample := range fixture.Cases {
		t.Run(sample.Name, func(t *testing.T) {
			projection, err := EvaluatePerspectiveProjection(
				EvaluatedCamera{PerspectiveDistance: sample.CameraDistance},
				EvaluatedTransform{Z: sample.ViewZ, Perspective: sample.ClipPerspective},
			)
			if err != nil {
				t.Fatalf("EvaluatePerspectiveProjection: %v", err)
			}
			if projection.ContractVersion != PerspectiveProjectionContractV1 {
				t.Fatalf("contract version = %q", projection.ContractVersion)
			}
			if projection.Source != sample.ExpectedSource {
				t.Fatalf("source = %q, want %q", projection.Source, sample.ExpectedSource)
			}
			assertProjectionClose(t, "distance", projection.Distance, sample.ExpectedDistance)
			assertProjectionClose(t, "origin_w", projection.OriginW, sample.ExpectedOriginW)
			for index := range projection.Matrix {
				assertProjectionClose(t, "matrix", projection.Matrix[index], sample.ExpectedMatrix[index])
			}
		})
	}
}

func TestEvaluatePerspectiveProjectionRejectsInvalidDistance(t *testing.T) {
	invalid := -100.0
	_, err := EvaluatePerspectiveProjection(
		EvaluatedCamera{PerspectiveDistance: 1000},
		EvaluatedTransform{Perspective: &invalid},
	)
	if err == nil {
		t.Fatal("expected negative clip perspective to fail closed")
	}
}

func loadPerspectiveProjectionFixture(t *testing.T) perspectiveProjectionFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve perspective projection fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "perspective-projection-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read perspective projection fixture: %v", err)
	}
	var fixture perspectiveProjectionFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode perspective projection fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func assertProjectionClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12f, want %.12f", label, got, want)
	}
}
