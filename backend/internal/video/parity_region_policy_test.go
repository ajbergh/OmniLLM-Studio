package video

import (
	"strings"
	"testing"
)

func TestParseParityRegionPolicyV1AndResolveByFrameIdentity(t *testing.T) {
	data := []byte(`{
  "version": 1,
  "canvas": {"width": 640, "height": 360},
  "frames": [
    {"frame_index": 30, "regions": [{"name": "title-baseline", "bounds": {"min_x": 100, "min_y": 20, "max_x": 540, "max_y": 80}}]},
    {"frame_index": 0, "regions": [{"name": "canvas-corner", "bounds": {"min_x": 0, "min_y": 0, "max_x": 16, "max_y": 16}}]}
  ]
}`)
	policy, evidence, err := ParseParityRegionPolicyV1(data)
	if err != nil {
		t.Fatalf("parse structural-region policy: %v", err)
	}
	if evidence.Version != 1 || evidence.CanvasWidth != 640 || evidence.CanvasHeight != 360 || evidence.ConfiguredFrames != 2 || len(evidence.SHA256) != 64 {
		t.Fatalf("unexpected evidence: %+v", evidence)
	}
	regions := RegionsForParityFrame(policy, 0)
	if len(regions) != 1 || regions[0].Name != "canvas-corner" {
		t.Fatalf("frame zero regions = %+v", regions)
	}
	regions[0].Name = "mutated"
	if RegionsForParityFrame(policy, 0)[0].Name != "canvas-corner" {
		t.Fatal("resolved policy regions alias stored policy state")
	}
	if got := RegionsForParityFrame(policy, 12); got != nil {
		t.Fatalf("unconfigured frame regions = %+v", got)
	}
}

func TestParityRegionPolicyV1RejectsAmbiguousAndOutOfCanvasState(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"unknown version", `{"version":2,"canvas":{"width":10,"height":10},"frames":[{"frame_index":0,"regions":[{"name":"a","bounds":{"min_x":0,"min_y":0,"max_x":1,"max_y":1}}]}]}`, "version=2"},
		{"unknown field", `{"version":1,"canvas":{"width":10,"height":10},"unexpected":true,"frames":[{"frame_index":0,"regions":[{"name":"a","bounds":{"min_x":0,"min_y":0,"max_x":1,"max_y":1}}]}]}`, "unknown field"},
		{"duplicate frame", `{"version":1,"canvas":{"width":10,"height":10},"frames":[{"frame_index":0,"regions":[{"name":"a","bounds":{"min_x":0,"min_y":0,"max_x":1,"max_y":1}}]},{"frame_index":0,"regions":[{"name":"b","bounds":{"min_x":1,"min_y":1,"max_x":2,"max_y":2}}]}]}`, "duplicate frame_index"},
		{"duplicate region", `{"version":1,"canvas":{"width":10,"height":10},"frames":[{"frame_index":0,"regions":[{"name":"a","bounds":{"min_x":0,"min_y":0,"max_x":1,"max_y":1}},{"name":"a","bounds":{"min_x":1,"min_y":1,"max_x":2,"max_y":2}}]}]}`, "duplicate region name"},
		{"outside canvas", `{"version":1,"canvas":{"width":10,"height":10},"frames":[{"frame_index":0,"regions":[{"name":"a","bounds":{"min_x":0,"min_y":0,"max_x":11,"max_y":1}}]}]}`, "exceeds 10x10 canvas"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ParseParityRegionPolicyV1([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
