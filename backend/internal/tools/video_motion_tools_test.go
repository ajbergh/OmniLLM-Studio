package tools

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/video"
)

func TestVideoMotionToolContracts(t *testing.T) {
	service := &video.Service{}
	definitions := []ToolDefinition{
		NewVideoProjectInspectTool(service).Definition(),
		NewVideoTimelineMutateTool(service).Definition(),
		NewVideoRenderFrameTool(service).Definition(),
		NewVideoRenderPreviewTool(service).Definition(),
		NewVideoRenderJobStatusTool(service).Definition(),
		NewVideoRenderJobCancelTool(service).Definition(),
	}
	seen := map[string]bool{}
	for _, definition := range definitions {
		if !definition.Enabled || definition.Name == "" || !json.Valid(definition.Parameters) {
			t.Errorf("invalid motion tool definition: %+v", definition)
		}
		if seen[definition.Name] {
			t.Errorf("duplicate motion tool name %q", definition.Name)
		}
		seen[definition.Name] = true
	}
	for _, name := range []string{"video_project_inspect", "video_timeline_mutate", "video_render_frame", "video_render_preview", "video_render_status", "video_render_cancel"} {
		if !seen[name] {
			t.Errorf("missing motion tool %q", name)
		}
	}
	if !NewVideoTimelineMutateTool(service).Definition().SideEffecting || !NewVideoRenderPreviewTool(service).Definition().SideEffecting {
		t.Fatal("mutating video tools must be governed as side-effecting")
	}
}

func TestDiagnosticRenderIterationLimit(t *testing.T) {
	tool := NewVideoRenderPreviewTool(&video.Service{})
	base := `{"project_id":"project","timeline_id":"timeline","revision":"sha256:test","iteration":%d}`
	for _, iteration := range []int{0, 4} {
		if err := tool.Validate(json.RawMessage([]byte(fmt.Sprintf(base, iteration)))); err == nil {
			t.Errorf("iteration %d should be rejected", iteration)
		}
	}
	if err := tool.Validate(json.RawMessage([]byte(fmt.Sprintf(base, 3)))); err != nil {
		t.Fatalf("third refinement should be accepted: %v", err)
	}
}
