package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type shapeStateFixture struct {
	Cases []struct {
		Name     string              `json:"name"`
		Input    TimelineV2Shape     `json:"input"`
		Expected EvaluatedShapeState `json:"expected"`
	} `json:"cases"`
}

func loadShapeStateFixture(t *testing.T) shapeStateFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve shape state fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "shape-state-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shape state fixture: %v", err)
	}
	var fixture shapeStateFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode shape state fixture: %v", err)
	}
	return fixture
}

func TestEvaluateShapeStateMatchesSharedFixture(t *testing.T) {
	fixture := loadShapeStateFixture(t)
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			state, err := EvaluateShapeState(&tc.Input)
			if err != nil {
				t.Fatalf("EvaluateShapeState: %v", err)
			}
			if state == nil || !reflect.DeepEqual(*state, tc.Expected) {
				got, _ := json.Marshal(state)
				want, _ := json.Marshal(tc.Expected)
				t.Fatalf("shape state\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestEvaluateShapeStateFailsClosedOnInvalidAuthoring(t *testing.T) {
	negative := -1.0
	nan := math.NaN()
	zero := 0
	cases := []struct {
		name  string
		shape TimelineV2Shape
		want  string
	}{
		{"kind", TimelineV2Shape{Kind: "triangle"}, "kind"},
		{"width", TimelineV2Shape{Kind: ShapeKindRectangle, Width: &zero}, "width"},
		{"negative-stroke", TimelineV2Shape{Kind: ShapeKindRectangle, StrokeWidth: &negative}, "stroke_width"},
		{"non-finite-blur", TimelineV2Shape{Kind: ShapeKindBlur, BlurRadius: &nan}, "blur_radius"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := EvaluateShapeState(&tc.shape); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
